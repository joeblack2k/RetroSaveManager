package main

import (
	"bytes"
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type saveRescanOptions struct {
	DryRun           bool
	PruneUnsupported bool
}

type saveRescanRejection struct {
	SaveID   string `json:"saveId"`
	Filename string `json:"filename"`
	Reason   string `json:"reason"`
}

type saveRescanResult struct {
	Scanned    int                   `json:"scanned"`
	Updated    int                   `json:"updated"`
	Rejected   int                   `json:"rejected"`
	Removed    int                   `json:"removed"`
	Rejections []saveRescanRejection `json:"rejections,omitempty"`
}

func runSaveRescan(args []string) error {
	fs := flag.NewFlagSet("rescan-saves", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "show intended changes without writing to disk")
	pruneUnsupported := fs.Bool("prune-unsupported", true, "remove unsupported/noise saves during rescan")
	if err := fs.Parse(args); err != nil {
		return err
	}

	a := newApp()
	if err := a.initSaveStore(); err != nil {
		return err
	}

	result, err := a.rescanSaves(saveRescanOptions{
		DryRun:           *dryRun,
		PruneUnsupported: *pruneUnsupported,
	})
	if err != nil {
		return err
	}

	log.Printf("save rescan complete: scanned=%d updated=%d rejected=%d removed=%d dry_run=%v prune_unsupported=%v",
		result.Scanned, result.Updated, result.Rejected, result.Removed, *dryRun, *pruneUnsupported)
	for _, rejected := range result.Rejections {
		log.Printf("rejected save: id=%s filename=%s reason=%s", rejected.SaveID, rejected.Filename, rejected.Reason)
	}
	return nil
}

func (a *app) rescanSaves(options saveRescanOptions) (saveRescanResult, error) {
	result := saveRescanResult{}
	records, err := a.rawSaveRecordsForRescan()
	if err != nil {
		return result, err
	}

	for _, original := range records {
		result.Scanned++

		payload, err := os.ReadFile(original.payloadPath)
		if err != nil {
			return result, fmt.Errorf("read payload for %s: %w", original.Summary.ID, err)
		}

		normalized := a.normalizeSaveInputDetailedWithOptions(saveCreateInput{
			Filename:      original.Summary.Filename,
			Payload:       payload,
			Game:          original.Summary.Game,
			Format:        original.Summary.Format,
			Metadata:      original.Summary.Metadata,
			ROMSHA1:       original.ROMSHA1,
			ROMMD5:        original.ROMMD5,
			SlotName:      original.SlotName,
			SystemSlug:    firstNonEmpty(original.SystemSlug, original.Summary.SystemSlug),
			GameSlug:      original.GameSlug,
			SystemPath:    original.SystemPath,
			GamePath:      original.GamePath,
			DisplayTitle:  original.Summary.DisplayTitle,
			RegionCode:    original.Summary.RegionCode,
			RegionFlag:    original.Summary.RegionFlag,
			LanguageCodes: original.Summary.LanguageCodes,
			CoverArtURL:   original.Summary.CoverArtURL,
			MemoryCard:    original.Summary.MemoryCard,
			Dreamcast:     original.Summary.Dreamcast,
			Saturn:        original.Summary.Saturn,
			Inspection:    original.Summary.Inspection,
			CreatedAt:     original.Summary.CreatedAt,
		}, normalizeSaveInputOptions{StoredSystemFallbackOnly: true})

		if normalized.Rejected || !isSupportedSystemSlug(normalized.Input.SystemSlug) {
			result.Rejected++
			result.Rejections = append(result.Rejections, saveRescanRejection{
				SaveID:   original.Summary.ID,
				Filename: original.Summary.Filename,
				Reason:   firstNonEmpty(normalized.RejectReason, normalized.Detection.Reason, errUnsupportedSaveFormat.Error()),
			})
			if options.PruneUnsupported {
				result.Removed++
				if !options.DryRun {
					if err := os.RemoveAll(original.dirPath); err != nil {
						return result, fmt.Errorf("remove unsupported save %s: %w", original.Summary.ID, err)
					}
					if a.saveStore != nil {
						cleanupEmptyParents(filepath.Dir(original.dirPath), a.saveStore.root)
					}
				}
			}
			continue
		}

		updated := applyNormalizedSaveToRecord(original, normalized.Input)
		changed, err := saveRecordChanged(original, updated)
		if err != nil {
			return result, err
		}
		if !changed {
			continue
		}

		result.Updated++
		if options.DryRun {
			continue
		}
		if err := relocateSaveRecordDir(a.saveStore.root, &updated); err != nil {
			return result, err
		}
		if err := persistSaveRecordMetadata(updated); err != nil {
			return result, err
		}
	}

	if !options.DryRun {
		if err := a.reloadSavesFromDisk(); err != nil {
			return result, err
		}
	}

	return result, nil
}

func (a *app) rawSaveRecordsForRescan() ([]saveRecord, error) {
	a.mu.Lock()
	store := a.saveStore
	a.mu.Unlock()
	if store == nil {
		return nil, fmt.Errorf("save store is not initialized")
	}
	return store.load()
}

func applyNormalizedSaveToRecord(record saveRecord, normalized saveCreateInput) saveRecord {
	updated := record
	systemSlug := canonicalSegment(normalized.SystemSlug, "unknown-system")
	updated.SystemSlug = systemSlug
	updated.SystemPath = strings.TrimSpace(normalized.SystemPath)
	if updated.SystemPath == "" && normalized.Game.System != nil {
		updated.SystemPath = sanitizeDisplayPathSegment(normalized.Game.System.Name, "Unknown System")
	}
	if updated.SystemPath == "" {
		updated.SystemPath = sanitizeDisplayPathSegment(systemSlug, "Unknown System")
	}

	updated.GamePath = strings.TrimSpace(normalized.GamePath)
	if updated.GamePath == "" {
		updated.GamePath = sanitizeDisplayPathSegment(normalized.DisplayTitle, "Unknown Game")
	}
	updated.GameSlug = canonicalSegment(firstNonEmpty(normalized.GameSlug, updated.GameSlug, normalized.DisplayTitle), "unknown-game")

	updated.Summary.DisplayTitle = normalized.DisplayTitle
	updated.Summary.SystemSlug = systemSlug
	updated.Summary.RegionCode = normalized.RegionCode
	updated.Summary.RegionFlag = regionFlagFromCode(normalized.RegionCode)
	updated.Summary.LanguageCodes = normalized.LanguageCodes
	updated.Summary.CoverArtURL = normalized.CoverArtURL
	updated.Summary.Metadata = normalized.Metadata
	updated.Summary.MemoryCard = normalized.MemoryCard
	updated.Summary.Dreamcast = normalized.Dreamcast
	updated.Summary.Saturn = normalized.Saturn
	updated.Summary.Inspection = normalized.Inspection
	updated.Summary.RuntimeProfile = strings.TrimSpace(normalized.RuntimeProfile)
	updated.Summary.CardSlot = strings.TrimSpace(normalized.CardSlot)
	updated.Summary.ProjectionID = strings.TrimSpace(normalized.ProjectionID)
	updated.Summary.SourceImportID = strings.TrimSpace(normalized.SourceImportID)
	updated.Summary.Portable = normalized.Portable
	updated.Summary.Game = normalized.Game
	updated.Summary.Game.DisplayTitle = normalized.DisplayTitle
	updated.Summary.Game.Name = normalized.DisplayTitle
	updated.Summary.Game.RegionCode = normalized.RegionCode
	updated.Summary.Game.RegionFlag = updated.Summary.RegionFlag
	updated.Summary.Game.LanguageCodes = normalized.LanguageCodes
	if updated.Summary.Game.System == nil && isSupportedSystemSlug(systemSlug) {
		updated.Summary.Game.System = supportedSystemFromSlug(systemSlug)
	}

	return updated
}

func saveRecordChanged(before, after saveRecord) (bool, error) {
	beforeJSON, err := json.Marshal(normalizeRecordForCompare(before))
	if err != nil {
		return false, fmt.Errorf("encode before record for compare: %w", err)
	}
	afterJSON, err := json.Marshal(normalizeRecordForCompare(after))
	if err != nil {
		return false, fmt.Errorf("encode after record for compare: %w", err)
	}
	return !bytes.Equal(beforeJSON, afterJSON), nil
}

func normalizeRecordForCompare(record saveRecord) saveRecord {
	clean := record
	clean.payloadPath = ""
	clean.dirPath = ""
	return clean
}

func persistSaveRecordMetadata(record saveRecord) error {
	metadataPath := filepath.Join(record.dirPath, "metadata.json")
	payload, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return fmt.Errorf("encode save metadata for %s: %w", record.Summary.ID, err)
	}
	if err := writeFileAtomic(metadataPath, payload, 0o644); err != nil {
		return fmt.Errorf("write save metadata for %s: %w", record.Summary.ID, err)
	}
	return nil
}

func relocateSaveRecordDir(saveRoot string, record *saveRecord) error {
	if record == nil {
		return nil
	}
	targetDir, err := safeJoinUnderRoot(saveRoot, record.SystemPath, record.GamePath, record.Summary.ID)
	if err != nil {
		return fmt.Errorf("build canonical save dir for %s: %w", record.Summary.ID, err)
	}
	if filepath.Clean(targetDir) != filepath.Clean(record.dirPath) {
		if err := applySaveLayoutMove(saveRoot, saveLayoutMove{
			SaveID: record.Summary.ID,
			From:   record.dirPath,
			To:     targetDir,
		}); err != nil {
			return err
		}
		record.dirPath = targetDir
	}
	record.payloadPath = filepath.Join(record.dirPath, record.PayloadFile)
	return nil
}
