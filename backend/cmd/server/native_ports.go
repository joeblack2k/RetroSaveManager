package main

import (
	"fmt"
	"path"
	"path/filepath"
	"strings"
)

const nativePortSystemSlug = "ports"

type nativePortMetadata struct {
	PortID           string `json:"portId,omitempty"`
	PortName         string `json:"portName,omitempty"`
	OriginSystemSlug string `json:"originSystemSlug,omitempty"`
	PortSaveKind     string `json:"portSaveKind,omitempty"`
	RelativePath     string `json:"relativePath,omitempty"`
	RootRelativePath string `json:"rootRelativePath,omitempty"`
	SlotID           string `json:"slotId,omitempty"`
	RuntimeProfile   string `json:"runtimeProfile,omitempty"`
	DisplayTitle     string `json:"displayTitle,omitempty"`
}

type nativePortManifest struct {
	ID               string
	Name             string
	OriginSystemSlug string
	RuntimeProfile   string
	AllowedPatterns  []string
}

var nativePortManifests = map[string]nativePortManifest{
	"ship-of-harkinian": {
		ID:               "ship-of-harkinian",
		Name:             "The Legend of Zelda: Ocarina of Time (Ship of Harkinian)",
		OriginSystemSlug: "n64",
		RuntimeProfile:   "port/ship-of-harkinian",
		AllowedPatterns:  []string{"Save/global.sav", "Save/file*.sav", "portable_home/Save/global.sav", "portable_home/Save/file*.sav"},
	},
	"starship": {
		ID:               "starship",
		Name:             "Star Fox 64 (Starship)",
		OriginSystemSlug: "n64",
		RuntimeProfile:   "port/starship",
		AllowedPatterns:  []string{"default.sav", "portable_home/default.sav"},
	},
	"spaghettikart": {
		ID:               "spaghettikart",
		Name:             "Mario Kart 64 (SpaghettiKart)",
		OriginSystemSlug: "n64",
		RuntimeProfile:   "port/spaghettikart",
		AllowedPatterns:  []string{"default.sav", "portable_home/default.sav"},
	},
	"super-metroid-native": {
		ID:               "super-metroid-native",
		Name:             "Super Metroid (Native Port)",
		OriginSystemSlug: "snes",
		RuntimeProfile:   "port/super-metroid-native",
		AllowedPatterns:  []string{"saves/*.srm", "portable_home/saves/*.srm"},
	},
	"sonic1-forever": {
		ID:               "sonic1-forever",
		Name:             "Sonic 1 Forever",
		OriginSystemSlug: "genesis",
		RuntimeProfile:   "port/sonic1-forever",
		AllowedPatterns: []string{
			"Scripts/Save/SaveSel.txt",
			"Scripts/Save/SaveSlot.txt",
			"portable_home/Scripts/Save/SaveSel.txt",
			"portable_home/Scripts/Save/SaveSlot.txt",
		},
	},
	"sonic3-air": {
		ID:               "sonic3-air",
		Name:             "Sonic 3 A.I.R.",
		OriginSystemSlug: "genesis",
		RuntimeProfile:   "port/sonic3-air",
		AllowedPatterns: []string{
			"saves/*.sav",
			"saves/*.srm",
			"saves/*.bin",
			"portable_home/saves/*.sav",
			"portable_home/saves/*.srm",
			"portable_home/saves/*.bin",
		},
	},
	"opengoal-jak1": {
		ID:               "opengoal-jak1",
		Name:             "Jak and Daxter: The Precursor Legacy (OpenGOAL)",
		OriginSystemSlug: "ps2",
		RuntimeProfile:   "port/opengoal-jak1",
	},
	"opengoal-jak2": {
		ID:               "opengoal-jak2",
		Name:             "Jak II (OpenGOAL)",
		OriginSystemSlug: "ps2",
		RuntimeProfile:   "port/opengoal-jak2",
	},
}

func nativePortMetadataFromForm(formValue func(string) string, runtimeProfile string) (nativePortMetadata, bool) {
	if formValue == nil {
		return nativePortMetadata{}, false
	}
	meta := nativePortMetadata{
		PortID:           canonicalOptionalSegment(formValue("portId")),
		PortName:         strings.TrimSpace(formValue("portName")),
		OriginSystemSlug: canonicalOptionalSegment(formValue("originSystemSlug")),
		PortSaveKind:     canonicalOptionalSegment(firstNonEmpty(formValue("portSaveKind"), "progress")),
		RelativePath:     normalizePortRelativePath(formValue("relativePath")),
		RootRelativePath: normalizePortRelativePath(formValue("rootRelativePath")),
		SlotID:           canonicalOptionalSegment(formValue("slotId")),
		RuntimeProfile:   strings.TrimSpace(runtimeProfile),
		DisplayTitle:     strings.TrimSpace(formValue("displayTitle")),
	}
	return meta, meta.PortID != "" || meta.RuntimeProfile != ""
}

func applyNativePortInput(input saveCreateInput, meta nativePortMetadata) saveCreateInput {
	if meta.PortID == "" && meta.RuntimeProfile == "" {
		return input
	}
	if manifest, ok := nativePortManifestForMetadata(meta); ok {
		if meta.PortID == "" {
			meta.PortID = manifest.ID
		}
		if meta.PortName == "" {
			meta.PortName = manifest.Name
		}
		if meta.OriginSystemSlug == "" {
			meta.OriginSystemSlug = manifest.OriginSystemSlug
		}
		if meta.RuntimeProfile == "" {
			meta.RuntimeProfile = manifest.RuntimeProfile
		}
	}
	input.PortID = meta.PortID
	input.PortName = meta.PortName
	input.OriginSystemSlug = meta.OriginSystemSlug
	input.PortSaveKind = firstNonEmpty(meta.PortSaveKind, "progress")
	input.RelativePath = meta.RelativePath
	input.RootRelativePath = meta.RootRelativePath
	input.SlotID = firstNonEmpty(meta.SlotID, canonicalSegment(strings.TrimSuffix(filepath.Base(meta.RelativePath), filepath.Ext(meta.RelativePath)), "default"))
	input.RuntimeProfile = meta.RuntimeProfile
	input.SourceArtifactProfile = meta.RuntimeProfile
	if input.DisplayTitle == "" {
		input.DisplayTitle = firstNonEmpty(meta.DisplayTitle, nativePortDisplayTitle(input))
	}
	if input.Game.Name == "" || input.Game.Name == strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename)) {
		input.Game.Name = input.DisplayTitle
		input.Game.DisplayTitle = input.DisplayTitle
	}
	input.Metadata = mergeRSMMetadata(input.Metadata, "nativePort", nativePortMetadataMap(input))
	return input
}

func nativePortMetadataMap(input saveCreateInput) map[string]any {
	return map[string]any{
		"portId":           strings.TrimSpace(input.PortID),
		"portName":         strings.TrimSpace(input.PortName),
		"originSystemSlug": strings.TrimSpace(input.OriginSystemSlug),
		"portSaveKind":     firstNonEmpty(strings.TrimSpace(input.PortSaveKind), "progress"),
		"relativePath":     strings.TrimSpace(input.RelativePath),
		"rootRelativePath": strings.TrimSpace(input.RootRelativePath),
		"slotId":           strings.TrimSpace(input.SlotID),
		"runtimeProfile":   strings.TrimSpace(input.RuntimeProfile),
		"displayTitle":     strings.TrimSpace(input.DisplayTitle),
	}
}

func nativePortDisplayTitle(input saveCreateInput) string {
	portName := strings.TrimSpace(input.PortName)
	if portName == "" {
		portName = strings.TrimSpace(input.PortID)
	}
	if portName == "" {
		portName = "Native Port"
	}
	slot := strings.TrimSpace(input.SlotID)
	if slot == "" {
		slot = strings.TrimSuffix(filepath.Base(input.RelativePath), filepath.Ext(input.RelativePath))
	}
	if slot == "" {
		return portName
	}
	return portName + " - " + nativePortSlotLabel(slot)
}

func nativePortSlotLabel(slot string) string {
	clean := strings.TrimSpace(strings.ReplaceAll(strings.ReplaceAll(slot, "_", " "), "-", " "))
	switch strings.ToLower(strings.ReplaceAll(clean, " ", "")) {
	case "global":
		return "Global"
	case "default":
		return "Default"
	case "savesel":
		return "Save Select"
	case "saveslot":
		return "Save Slot"
	}
	if strings.HasPrefix(strings.ToLower(clean), "file") && len(clean) > 4 {
		return "File " + strings.TrimSpace(clean[4:])
	}
	return titleizeNativePortSlot(clean)
}

func titleizeNativePortSlot(value string) string {
	words := strings.Fields(value)
	if len(words) == 0 {
		return "Default"
	}
	for index, word := range words {
		if word == "" {
			continue
		}
		words[index] = strings.ToUpper(word[:1]) + word[1:]
	}
	return strings.Join(words, " ")
}

func validateNativePortSave(input saveCreateInput, detection saveSystemDetectionResult) consoleValidationResult {
	if !detection.Evidence.HelperTrusted && !metadataHasTrustedSystemEvidence(input.Metadata) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: "native port saves must come from a trusted helper manifest",
		}
	}
	meta := nativePortMetadataFromInput(input)
	manifest, ok := nativePortManifestForMetadata(meta)
	if !ok {
		return consoleValidationResult{Rejected: true, RejectReason: "unknown native port manifest"}
	}
	if canonicalOptionalSegment(meta.PortSaveKind) != "progress" {
		return consoleValidationResult{Rejected: true, RejectReason: "only native port progress saves are supported"}
	}
	if len(input.Payload) == 0 {
		return consoleValidationResult{Rejected: true, RejectReason: "native port save payload is empty"}
	}
	if len(input.Payload) > 16*1024*1024 {
		return consoleValidationResult{Rejected: true, RejectReason: "native port save payload is too large"}
	}
	if !nativePortPathAllowed(manifest, meta.RelativePath) {
		return consoleValidationResult{
			Rejected:     true,
			RejectReason: fmt.Sprintf("native port path %q is not allowed for %s", meta.RelativePath, manifest.ID),
		}
	}
	if meta.RootRelativePath != "" && !safePortRelativePath(meta.RootRelativePath) {
		return consoleValidationResult{Rejected: true, RejectReason: "native port root-relative path is unsafe"}
	}
	return consoleValidationResult{
		Inspection: &saveInspection{
			ParserLevel:        saveParserLevelContainer,
			ParserID:           "native-port-manifest",
			ValidatedSystem:    nativePortSystemSlug,
			ValidatedGameID:    manifest.ID,
			ValidatedGameTitle: nativePortDisplayTitle(input),
			TrustLevel:         validationTrustLevel(saveParserLevelContainer),
			Evidence: []string{
				"trusted helper native port manifest",
				"portId=" + manifest.ID,
				"relativePath=" + meta.RelativePath,
			},
			PayloadSizeBytes: len(input.Payload),
			SemanticFields: map[string]any{
				"originSystemSlug": manifest.OriginSystemSlug,
				"portSaveKind":     meta.PortSaveKind,
				"slotId":           meta.SlotID,
				"runtimeProfile":   manifest.RuntimeProfile,
			},
		},
	}
}

func nativePortMetadataFromInput(input saveCreateInput) nativePortMetadata {
	return nativePortMetadata{
		PortID:           strings.TrimSpace(input.PortID),
		PortName:         strings.TrimSpace(input.PortName),
		OriginSystemSlug: canonicalOptionalSegment(input.OriginSystemSlug),
		PortSaveKind:     canonicalOptionalSegment(firstNonEmpty(input.PortSaveKind, "progress")),
		RelativePath:     normalizePortRelativePath(input.RelativePath),
		RootRelativePath: normalizePortRelativePath(input.RootRelativePath),
		SlotID:           canonicalOptionalSegment(input.SlotID),
		RuntimeProfile:   strings.TrimSpace(input.RuntimeProfile),
		DisplayTitle:     strings.TrimSpace(input.DisplayTitle),
	}
}

func nativePortManifestForMetadata(meta nativePortMetadata) (nativePortManifest, bool) {
	if meta.PortID != "" {
		manifest, ok := nativePortManifests[meta.PortID]
		return manifest, ok
	}
	profile := strings.TrimSpace(meta.RuntimeProfile)
	for _, manifest := range nativePortManifests {
		if profile != "" && strings.EqualFold(manifest.RuntimeProfile, profile) {
			return manifest, true
		}
	}
	return nativePortManifest{}, false
}

func nativePortPathAllowed(manifest nativePortManifest, relativePath string) bool {
	relativePath = normalizePortRelativePath(relativePath)
	if !safePortRelativePath(relativePath) {
		return false
	}
	for _, pattern := range manifest.AllowedPatterns {
		if ok, _ := path.Match(pattern, relativePath); ok {
			return true
		}
	}
	return false
}

func normalizePortRelativePath(raw string) string {
	value := strings.TrimSpace(strings.ReplaceAll(raw, "\\", "/"))
	value = path.Clean(value)
	if value == "." {
		return ""
	}
	return strings.TrimPrefix(value, "./")
}

func safePortRelativePath(value string) bool {
	value = normalizePortRelativePath(value)
	return value != "" && !strings.HasPrefix(value, "../") && !strings.Contains(value, "/../") && !path.IsAbs(value)
}
