package main

import (
	"flag"
	"fmt"
	"log"
	"os"
	"path/filepath"
	"strings"
)

type playStationBackfillOptions struct {
	DryRun               bool
	ReplaceRaw           bool
	PSXProfile           string
	PS2Profile           string
	DefaultPSXCardSlot   string
	DefaultPS2CardSlot   string
}

type playStationBackfillResult struct {
	Scanned   int                           `json:"scanned"`
	Candidates int                          `json:"candidates"`
	Migrated  int                           `json:"migrated"`
	Removed   int                           `json:"removed"`
	Skipped   int                           `json:"skipped"`
	Failures  []playStationBackfillFailure  `json:"failures,omitempty"`
}

type playStationBackfillFailure struct {
	SaveID   string `json:"saveId"`
	Filename string `json:"filename"`
	Reason   string `json:"reason"`
}

func runPlayStationBackfill(args []string) error {
	fs := flag.NewFlagSet("backfill-playstation", flag.ContinueOnError)
	dryRun := fs.Bool("dry-run", false, "inspect PlayStation legacy records without writing changes")
	replaceRaw := fs.Bool("replace-raw", true, "remove migrated raw PlayStation card records after projection records are created")
	psxProfile := fs.String("psx-profile", "", "explicit runtime profile for legacy PS1 cards (for example psx/mister or psx/retroarch)")
	ps2Profile := fs.String("ps2-profile", "ps2/pcsx2", "runtime profile for legacy PS2 cards")
	defaultPSXCardSlot := fs.String("default-psx-card-slot", "", "explicit card slot to use when legacy PS1 records do not encode Memory Card 1/2 in filename or slot")
	defaultPS2CardSlot := fs.String("default-ps2-card-slot", "", "explicit card slot to use when legacy PS2 records do not encode Memory Card 1/2 in filename or slot")
	if err := fs.Parse(args); err != nil {
		return err
	}

	a := newApp()
	if err := a.initSaveStore(); err != nil {
		return err
	}

	result, err := a.backfillPlayStation(playStationBackfillOptions{
		DryRun:             *dryRun,
		ReplaceRaw:         *replaceRaw,
		PSXProfile:         strings.TrimSpace(*psxProfile),
		PS2Profile:         strings.TrimSpace(*ps2Profile),
		DefaultPSXCardSlot: strings.TrimSpace(*defaultPSXCardSlot),
		DefaultPS2CardSlot: strings.TrimSpace(*defaultPS2CardSlot),
	})
	if err != nil {
		return err
	}

	log.Printf("playstation backfill complete: scanned=%d candidates=%d migrated=%d removed=%d skipped=%d dry_run=%v replace_raw=%v", result.Scanned, result.Candidates, result.Migrated, result.Removed, result.Skipped, *dryRun, *replaceRaw)
	for _, failure := range result.Failures {
		log.Printf("playstation backfill skipped: id=%s filename=%s reason=%s", failure.SaveID, failure.Filename, failure.Reason)
	}
	return nil
}

func (a *app) backfillPlayStation(options playStationBackfillOptions) (playStationBackfillResult, error) {
	result := playStationBackfillResult{}
	records, err := a.rawSaveRecordsForRescan()
	if err != nil {
		return result, err
	}

	for _, record := range records {
		result.Scanned++
		if _, _, _, ok := playStationProjectionInfoFromRecord(record); ok {
			result.Skipped++
			continue
		}
		payload, err := os.ReadFile(record.payloadPath)
		if err != nil {
			return result, fmt.Errorf("read payload for %s: %w", record.Summary.ID, err)
		}
		artifactKind := classifyPlayStationArtifact(record.Summary.Game.System, record.Summary.Format, record.Summary.Filename, payload)
		if artifactKind != saveArtifactPS1MemoryCard && artifactKind != saveArtifactPS2MemoryCard {
			result.Skipped++
			continue
		}
		result.Candidates++

		runtimeProfile, systemSlug, err := selectLegacyPlayStationRuntimeProfile(record, artifactKind, options)
		if err != nil {
			result.Failures = append(result.Failures, playStationBackfillFailure{SaveID: record.Summary.ID, Filename: record.Summary.Filename, Reason: err.Error()})
			continue
		}
		cardSlot, err := selectLegacyPlayStationCardSlot(record, artifactKind, options)
		if err != nil {
			result.Failures = append(result.Failures, playStationBackfillFailure{SaveID: record.Summary.ID, Filename: record.Summary.Filename, Reason: err.Error()})
			continue
		}
		if options.DryRun {
			result.Migrated++
			continue
		}

		preview := a.normalizeSaveInputDetailedWithOptions(saveCreateInput{
			Filename:      record.Summary.Filename,
			Payload:       payload,
			Game:          record.Summary.Game,
			Format:        record.Summary.Format,
			Metadata:      record.Summary.Metadata,
			ROMSHA1:       record.ROMSHA1,
			ROMMD5:        record.ROMMD5,
			SlotName:      cardSlot,
			SystemSlug:    firstNonEmpty(record.SystemSlug, systemSlug),
			GameSlug:      record.GameSlug,
			SystemPath:    record.SystemPath,
			GamePath:      record.GamePath,
			DisplayTitle:  record.Summary.DisplayTitle,
			RegionCode:    record.Summary.RegionCode,
			RegionFlag:    record.Summary.RegionFlag,
			LanguageCodes: record.Summary.LanguageCodes,
			CoverArtURL:   record.Summary.CoverArtURL,
			MemoryCard:    record.Summary.MemoryCard,
			CreatedAt:     record.Summary.CreatedAt,
		}, normalizeSaveInputOptions{StoredSystemFallbackOnly: true})
		if preview.Rejected {
			result.Failures = append(result.Failures, playStationBackfillFailure{SaveID: record.Summary.ID, Filename: record.Summary.Filename, Reason: firstNonEmpty(preview.RejectReason, "legacy PlayStation record rejected during backfill")})
			continue
		}
		_, conflict, err := a.createPlayStationProjectionSave(saveCreateInput{
			Filename:      record.Summary.Filename,
			Payload:       payload,
			Game:          record.Summary.Game,
			Format:        record.Summary.Format,
			Metadata:      record.Summary.Metadata,
			ROMSHA1:       record.ROMSHA1,
			ROMMD5:        record.ROMMD5,
			SlotName:      cardSlot,
			SystemSlug:    systemSlug,
			GameSlug:      record.GameSlug,
			SystemPath:    record.SystemPath,
			GamePath:      record.GamePath,
			DisplayTitle:  record.Summary.DisplayTitle,
			RegionCode:    record.Summary.RegionCode,
			RegionFlag:    record.Summary.RegionFlag,
			LanguageCodes: record.Summary.LanguageCodes,
			CoverArtURL:   record.Summary.CoverArtURL,
			MemoryCard:    record.Summary.MemoryCard,
			CreatedAt:     record.Summary.CreatedAt,
		}, preview, runtimeDeviceTypeFromProfile(runtimeProfile), "backfill:"+record.Summary.ID)
		if err != nil {
			result.Failures = append(result.Failures, playStationBackfillFailure{SaveID: record.Summary.ID, Filename: record.Summary.Filename, Reason: err.Error()})
			continue
		}
		if conflict != nil {
			result.Failures = append(result.Failures, playStationBackfillFailure{SaveID: record.Summary.ID, Filename: record.Summary.Filename, Reason: "unexpected backfill conflict for projection line " + conflict.ConflictKey})
			continue
		}
		result.Migrated++
		if options.ReplaceRaw {
			if err := os.RemoveAll(record.dirPath); err != nil {
				return result, fmt.Errorf("remove legacy playstation save %s: %w", record.Summary.ID, err)
			}
			if a.saveStore != nil {
				cleanupEmptyParents(filepath.Dir(record.dirPath), a.saveStore.root)
			}
			result.Removed++
		}
	}

	if !options.DryRun {
		if err := a.reloadSavesFromDisk(); err != nil {
			return result, err
		}
	}
	return result, nil
}

func selectLegacyPlayStationRuntimeProfile(record saveRecord, artifactKind saveArtifactKind, options playStationBackfillOptions) (string, string, error) {
	if runtimeProfile := existingRuntimeProfileForLegacyRecord(record); runtimeProfile != "" {
		profile, systemSlug, err := supportedPlayStationRuntimeProfile(runtimeDeviceTypeFromProfile(runtimeProfile), artifactKind)
		if err == nil {
			return profile, systemSlug, nil
		}
	}
	switch artifactKind {
	case saveArtifactPS1MemoryCard:
		if strings.TrimSpace(options.PSXProfile) == "" {
			return "", "", fmt.Errorf("legacy PS1 card requires explicit --psx-profile")
		}
		profile, systemSlug, err := supportedPlayStationRuntimeProfile(runtimeDeviceTypeFromProfile(options.PSXProfile), artifactKind)
		if err != nil {
			return "", "", err
		}
		return profile, systemSlug, nil
	case saveArtifactPS2MemoryCard:
		profileValue := strings.TrimSpace(options.PS2Profile)
		if profileValue == "" {
			profileValue = "ps2/pcsx2"
		}
		profile, systemSlug, err := supportedPlayStationRuntimeProfile(runtimeDeviceTypeFromProfile(profileValue), artifactKind)
		if err != nil {
			return "", "", err
		}
		return profile, systemSlug, nil
	default:
		return "", "", fmt.Errorf("record is not a supported PlayStation card artifact")
	}
}

func existingRuntimeProfileForLegacyRecord(record saveRecord) string {
	if strings.TrimSpace(record.Summary.RuntimeProfile) != "" {
		return strings.TrimSpace(record.Summary.RuntimeProfile)
	}
	meta, ok := record.Summary.Metadata.(map[string]any)
	if !ok {
		return ""
	}
	playstation, ok := meta["playstation"].(map[string]any)
	if !ok {
		return ""
	}
	if runtimeProfile, ok := playstation["runtimeProfile"].(string); ok {
		return strings.TrimSpace(runtimeProfile)
	}
	return ""
}

func selectLegacyPlayStationCardSlot(record saveRecord, artifactKind saveArtifactKind, options playStationBackfillOptions) (string, error) {
	if cardSlot, ok := deriveExplicitMemoryCardName(record.Summary.CardSlot, record.Summary.Filename); ok {
		return cardSlot, nil
	}
	if cardSlot, ok := deriveExplicitMemoryCardName(record.SlotName, record.Summary.Filename); ok {
		return cardSlot, nil
	}
	switch artifactKind {
	case saveArtifactPS1MemoryCard:
		if strings.TrimSpace(options.DefaultPSXCardSlot) != "" {
			if cardSlot, ok := deriveExplicitMemoryCardName(options.DefaultPSXCardSlot, options.DefaultPSXCardSlot); ok {
				return cardSlot, nil
			}
			return "", fmt.Errorf("invalid --default-psx-card-slot value %q", options.DefaultPSXCardSlot)
		}
		return "", fmt.Errorf("legacy PS1 card requires explicit Memory Card 1/2 slot")
	case saveArtifactPS2MemoryCard:
		if strings.TrimSpace(options.DefaultPS2CardSlot) != "" {
			if cardSlot, ok := deriveExplicitMemoryCardName(options.DefaultPS2CardSlot, options.DefaultPS2CardSlot); ok {
				return cardSlot, nil
			}
			return "", fmt.Errorf("invalid --default-ps2-card-slot value %q", options.DefaultPS2CardSlot)
		}
		return "", fmt.Errorf("legacy PS2 card requires explicit Memory Card 1/2 slot")
	default:
		return "", fmt.Errorf("record is not a supported PlayStation card artifact")
	}
}
