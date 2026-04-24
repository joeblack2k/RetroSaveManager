package main

import (
	"crypto/sha256"
	"encoding/hex"
	"strings"
)

type uploadDuplicateDisposition string

const (
	uploadDuplicateNone            uploadDuplicateDisposition = ""
	uploadDuplicateIgnoredLatest   uploadDuplicateDisposition = "ignored-latest"
	uploadDuplicateStaleHistorical uploadDuplicateDisposition = "stale-historical"
)

type uploadDuplicateCheck struct {
	Disposition uploadDuplicateDisposition
	Latest      saveRecord
	Matching    saveRecord
	Found       bool
}

func payloadSHA256Hex(payload []byte) string {
	sum := sha256.Sum256(payload)
	return hex.EncodeToString(sum[:])
}

func latestSaveRecordForCanonicalKey(records []saveRecord, key string) (saveRecord, bool) {
	return latestSaveRecordForKey(records, key, canonicalVersionKeyForRecord)
}

func latestSaveRecordForKey(records []saveRecord, key string, keyForRecord func(saveRecord) string) (saveRecord, bool) {
	cleanKey := strings.TrimSpace(key)
	if cleanKey == "" || keyForRecord == nil {
		return saveRecord{}, false
	}
	var latest saveRecord
	found := false
	for _, record := range records {
		if strings.TrimSpace(keyForRecord(record)) != cleanKey {
			continue
		}
		if !saveRecordPayloadExists(record) {
			continue
		}
		if !found || saveRecordSortsAfter(record, latest) {
			latest = record
			found = true
		}
	}
	return latest, found
}

func canonicalDuplicateTrackKeyForRecord(record saveRecord) string {
	track := canonicalTrackFromRecord(record)
	if canonicalTrackTitleKey(track.DisplayTitle) == "unknown game" {
		return ""
	}
	return canonicalTrackKey(track)
}

func canonicalDuplicateTrackKeyForInput(input saveCreateInput, filename string) string {
	if strings.TrimSpace(input.Filename) == "" {
		input.Filename = filename
	}
	track := canonicalTrackFromInput(input)
	if canonicalTrackTitleKey(track.DisplayTitle) == "unknown game" {
		return ""
	}
	return canonicalTrackKey(track)
}

func saveRecordSortsAfter(left, right saveRecord) bool {
	if left.Summary.Version != right.Summary.Version {
		return left.Summary.Version > right.Summary.Version
	}
	if !left.Summary.CreatedAt.Equal(right.Summary.CreatedAt) {
		return left.Summary.CreatedAt.After(right.Summary.CreatedAt)
	}
	return strings.TrimSpace(left.Summary.ID) > strings.TrimSpace(right.Summary.ID)
}

func checkUploadDuplicate(records []saveRecord, input saveCreateInput) uploadDuplicateCheck {
	filename := safeFilename(firstNonEmpty(input.Filename, "save.bin"))
	shaHex := payloadSHA256Hex(input.Payload)

	type duplicateScope struct {
		name   string
		key    string
		keyFor func(saveRecord) string
	}
	scopes := []duplicateScope{{
		name:   "version",
		key:    canonicalVersionKeyForInput(input, filename),
		keyFor: canonicalVersionKeyForRecord,
	}}
	if trackKey := canonicalDuplicateTrackKeyForInput(input, filename); trackKey != "" {
		scopes = append(scopes, duplicateScope{
			name:   "track",
			key:    trackKey,
			keyFor: canonicalDuplicateTrackKeyForRecord,
		})
	}

	seenScopes := map[string]struct{}{}
	for _, scope := range scopes {
		scopeKey := strings.TrimSpace(scope.key)
		if scopeKey == "" {
			continue
		}
		seenKey := scope.name + ":" + scopeKey
		if _, seen := seenScopes[seenKey]; seen {
			continue
		}
		seenScopes[seenKey] = struct{}{}

		latest, hasLatest := latestSaveRecordForKey(records, scopeKey, scope.keyFor)
		if !hasLatest {
			continue
		}

		var matching saveRecord
		foundMatch := false
		for _, record := range records {
			if strings.TrimSpace(scope.keyFor(record)) != scopeKey {
				continue
			}
			if !saveRecordPayloadExists(record) {
				continue
			}
			if strings.TrimSpace(record.Summary.SHA256) != shaHex {
				continue
			}
			if !foundMatch || saveRecordSortsAfter(record, matching) {
				matching = record
				foundMatch = true
			}
		}
		if !foundMatch {
			continue
		}
		if strings.TrimSpace(latest.Summary.SHA256) == shaHex {
			return uploadDuplicateCheck{Disposition: uploadDuplicateIgnoredLatest, Latest: latest, Matching: latest, Found: true}
		}
		return uploadDuplicateCheck{Disposition: uploadDuplicateStaleHistorical, Latest: latest, Matching: matching, Found: true}
	}
	return uploadDuplicateCheck{}
}

func duplicateUploadResponse(record saveRecord, disposition uploadDuplicateDisposition) map[string]any {
	return map[string]any{
		"success":              true,
		"duplicate":            true,
		"duplicateDisposition": string(disposition),
		"save": map[string]any{
			"id":      record.Summary.ID,
			"sha256":  record.Summary.SHA256,
			"version": record.Summary.Version,
		},
	}
}

func staleHistoricalUploadResponse(latest saveRecord, reason string) map[string]any {
	if strings.TrimSpace(reason) == "" {
		reason = "stale_historical_duplicate"
	}
	return map[string]any{
		"success": false,
		"reason":  reason,
		"latest": map[string]any{
			"id":      latest.Summary.ID,
			"sha256":  latest.Summary.SHA256,
			"version": latest.Summary.Version,
		},
	}
}

func metadataHasRollbackAudit(metadata any) bool {
	meta, ok := metadata.(map[string]any)
	if !ok || meta == nil {
		return false
	}
	_, ok = meta["rollback"]
	return ok
}

func saveRecordIsRollback(record saveRecord) bool {
	return metadataHasRollbackAudit(record.Summary.Metadata)
}

func saveRecordRevisionIdentity(record saveRecord) string {
	id := strings.TrimSpace(record.Summary.ID)
	if saveRecordIsRollback(record) {
		return "rollback:" + id
	}
	if sha := strings.TrimSpace(record.Summary.SHA256); sha != "" {
		return "sha:" + sha
	}
	return "id:" + id
}

func saveSummaryRevisionIdentity(summary saveSummary) string {
	id := strings.TrimSpace(summary.ID)
	if metadataHasRollbackAudit(summary.Metadata) {
		return "rollback:" + id
	}
	if sha := strings.TrimSpace(summary.SHA256); sha != "" {
		return "sha:" + sha
	}
	return "id:" + id
}

func dedupeSaveSummaryRevisions(summaries []saveSummary) []saveSummary {
	if len(summaries) <= 1 {
		return summaries
	}
	seen := map[string]struct{}{}
	deduped := make([]saveSummary, 0, len(summaries))
	for _, summary := range summaries {
		key := saveSummaryRevisionIdentity(summary)
		if _, exists := seen[key]; exists {
			continue
		}
		seen[key] = struct{}{}
		deduped = append(deduped, summary)
	}
	return deduped
}

func psLogicalRevisionIsRollback(revision psLogicalSaveRevision) bool {
	return strings.HasPrefix(strings.TrimSpace(revision.ImportID), "rollback:")
}

func n64ControllerPakRevisionIsRollback(revision n64ControllerPakLogicalRevision) bool {
	return strings.HasPrefix(strings.TrimSpace(revision.ImportID), "rollback:")
}
