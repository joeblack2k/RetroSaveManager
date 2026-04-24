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
	cleanKey := strings.TrimSpace(key)
	var latest saveRecord
	found := false
	for _, record := range records {
		if strings.TrimSpace(canonicalVersionKeyForRecord(record)) != cleanKey {
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
	key := canonicalVersionKeyForInput(input, filename)
	shaHex := payloadSHA256Hex(input.Payload)
	latest, hasLatest := latestSaveRecordForCanonicalKey(records, key)
	if !hasLatest {
		return uploadDuplicateCheck{}
	}

	var matching saveRecord
	foundMatch := false
	for _, record := range records {
		if strings.TrimSpace(canonicalVersionKeyForRecord(record)) != strings.TrimSpace(key) {
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
		return uploadDuplicateCheck{}
	}
	if strings.TrimSpace(latest.Summary.SHA256) == shaHex {
		return uploadDuplicateCheck{Disposition: uploadDuplicateIgnoredLatest, Latest: latest, Matching: latest, Found: true}
	}
	return uploadDuplicateCheck{Disposition: uploadDuplicateStaleHistorical, Latest: latest, Matching: matching, Found: true}
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

func psLogicalRevisionIsRollback(revision psLogicalSaveRevision) bool {
	return strings.HasPrefix(strings.TrimSpace(revision.ImportID), "rollback:")
}

func n64ControllerPakRevisionIsRollback(revision n64ControllerPakLogicalRevision) bool {
	return strings.HasPrefix(strings.TrimSpace(revision.ImportID), "rollback:")
}
