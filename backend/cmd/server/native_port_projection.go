package main

import (
	"fmt"
	"path/filepath"
	"strings"
)

type nativePortProjectionAdapter struct {
	PortID             string
	RuntimeProfile     string
	OriginSystemSlug   string
	OriginDisplayTitle string
	CanonicalMediaType string
	TargetFilename     string
	OriginIdentity     bool
	ValidatePortSave   func([]byte) error
	PortToN64Canonical func(saveCreateInput) ([]byte, error)
	N64CanonicalToPort func(saveSummary, []byte) (string, []byte, error)
}

type nativePortOriginKey struct {
	SystemSlug string
	TitleKey   string
	MediaType  string
}

var nativePortProjectionAdapters = []nativePortProjectionAdapter{
	{
		PortID:             "ship-of-harkinian",
		RuntimeProfile:     "port/ship-of-harkinian",
		OriginSystemSlug:   "n64",
		OriginDisplayTitle: "The Legend of Zelda: Ocarina of Time",
		CanonicalMediaType: "sram",
		TargetFilename:     "file1.sav",
		PortToN64Canonical: func(input saveCreateInput) ([]byte, error) {
			return shipOfHarkinianToOOTSRAM(input.Payload, firstNonEmpty(input.SlotID, input.SlotName, input.RelativePath, input.Filename))
		},
		N64CanonicalToPort: func(summary saveSummary, payload []byte) (string, []byte, error) {
			return ootSRAMToShipOfHarkinian(summary, payload)
		},
	},
	{
		PortID:             "starship",
		RuntimeProfile:     "port/starship",
		OriginSystemSlug:   "n64",
		OriginDisplayTitle: "Star Fox 64",
		CanonicalMediaType: "eeprom",
		TargetFilename:     "default.sav",
		ValidatePortSave: func(payload []byte) error {
			_, err := parseSF64EEPROM(payload)
			return err
		},
	},
	{
		PortID:             "spaghettikart",
		RuntimeProfile:     "port/spaghettikart",
		OriginSystemSlug:   "n64",
		OriginDisplayTitle: "Mario Kart 64",
		CanonicalMediaType: "eeprom",
		TargetFilename:     "default.sav",
		ValidatePortSave: func(payload []byte) error {
			_, err := parseMK64EEPROM(payload)
			return err
		},
	},
	{
		PortID:             "super-metroid-native",
		RuntimeProfile:     "port/super-metroid-native",
		OriginSystemSlug:   "snes",
		OriginDisplayTitle: "Super Metroid",
		TargetFilename:     "sm.srm",
		OriginIdentity:     true,
		ValidatePortSave: func(payload []byte) error {
			if len(payload) == 0 {
				return fmt.Errorf("Super Metroid native port save is empty")
			}
			return nil
		},
	},
}

var nativePortProjectionAdaptersByProfile = func() map[string]nativePortProjectionAdapter {
	out := make(map[string]nativePortProjectionAdapter, len(nativePortProjectionAdapters))
	for _, adapter := range nativePortProjectionAdapters {
		out[nativePortProfileKey(adapter.RuntimeProfile)] = adapter
	}
	return out
}()

var nativePortProjectionAdaptersByOrigin = func() map[nativePortOriginKey]nativePortProjectionAdapter {
	out := make(map[nativePortOriginKey]nativePortProjectionAdapter, len(nativePortProjectionAdapters))
	for _, adapter := range nativePortProjectionAdapters {
		if adapter.OriginSystemSlug == "n64" {
			out[nativePortOriginKeyForAdapter(adapter, adapter.CanonicalMediaType)] = adapter
			continue
		}
		if adapter.OriginIdentity {
			out[nativePortOriginKeyForAdapter(adapter, "")] = adapter
		}
	}
	return out
}()

func nativePortProfileKey(profile string) string {
	return strings.ToLower(strings.TrimSpace(profile))
}

func nativePortRuntimeProfileFromID(portID string) string {
	portID = canonicalOptionalSegment(portID)
	if portID == "" {
		return ""
	}
	return "port/" + portID
}

func nativePortOriginKeyForAdapter(adapter nativePortProjectionAdapter, mediaType string) nativePortOriginKey {
	return nativePortOriginKey{
		SystemSlug: canonicalSegment(adapter.OriginSystemSlug, ""),
		TitleKey:   canonicalTrackTitleKey(adapter.OriginDisplayTitle),
		MediaType:  canonicalOptionalSegment(mediaType),
	}
}

func nativePortOriginKeyForSummary(summary saveSummary, mediaType string) nativePortOriginKey {
	return nativePortOriginKey{
		SystemSlug: canonicalSegment(saveSummarySystemSlug(summary), ""),
		TitleKey:   canonicalTrackTitleKey(firstNonEmpty(summary.DisplayTitle, summary.Game.DisplayTitle, summary.Game.Name)),
		MediaType:  canonicalOptionalSegment(mediaType),
	}
}

func nativePortManifestForRuntimeProfile(profile string) (nativePortManifest, bool) {
	manifest, ok := nativePortManifestsByRuntimeProfile[nativePortProfileKey(profile)]
	return manifest, ok
}

func nativePortProjectionAdapterForProfile(profile string) (nativePortProjectionAdapter, bool) {
	adapter, ok := nativePortProjectionAdaptersByProfile[nativePortProfileKey(profile)]
	return adapter, ok
}

func nativePortProjectionAdapterForCandidates(runtimeProfile, portID string) (nativePortProjectionAdapter, bool) {
	if adapter, ok := nativePortProjectionAdapterForProfile(runtimeProfile); ok {
		return adapter, true
	}
	if adapter, ok := nativePortProjectionAdapterForProfile(nativePortRuntimeProfileFromID(portID)); ok {
		return adapter, true
	}
	return nativePortProjectionAdapter{}, false
}

func nativePortProjectionAdapterForSummary(summary saveSummary) (nativePortProjectionAdapter, bool) {
	if adapter, ok := nativePortProjectionAdapterForCandidates(summary.RuntimeProfile, summary.PortID); ok {
		return adapter, true
	}
	return nativePortProjectionAdapterForOriginSummary(summary)
}

func nativePortProjectionAdapterForN64Summary(summary saveSummary) (nativePortProjectionAdapter, bool) {
	if canonicalSegment(saveSummarySystemSlug(summary), "") != "n64" {
		return nativePortProjectionAdapter{}, false
	}
	info, ok := n64SummaryMediaInfo(summary)
	if !ok {
		return nativePortProjectionAdapter{}, false
	}
	adapter, ok := nativePortProjectionAdaptersByOrigin[nativePortOriginKeyForSummary(summary, info.MediaType)]
	return adapter, ok
}

func nativePortProjectionAdapterForOriginSummary(summary saveSummary) (nativePortProjectionAdapter, bool) {
	systemSlug := canonicalSegment(saveSummarySystemSlug(summary), "")
	if systemSlug == "n64" {
		return nativePortProjectionAdapterForN64Summary(summary)
	}
	if systemSlug == "" || systemSlug == nativePortSystemSlug {
		return nativePortProjectionAdapter{}, false
	}
	adapter, ok := nativePortProjectionAdaptersByOrigin[nativePortOriginKeyForSummary(summary, "")]
	return adapter, ok
}

func nativePortRuntimeProfilesForSummary(summary saveSummary) []runtimeProfileDefinition {
	if profile, ok := storedOriginNativePortProfile(summary); ok {
		if definition, exists := runtimeProfilesByID[profile]; exists {
			return []runtimeProfileDefinition{definition}
		}
	}
	if adapter, ok := nativePortProjectionAdapterForOriginSummary(summary); ok {
		if definition, exists := runtimeProfilesByID[adapter.RuntimeProfile]; exists {
			return []runtimeProfileDefinition{definition}
		}
	}
	return nil
}

func originRuntimeProfilesForNativePortSummary(summary saveSummary) []runtimeProfileDefinition {
	if canonicalSegment(saveSummarySystemSlug(summary), "") != nativePortSystemSlug {
		return nil
	}
	adapter, ok := nativePortProjectionAdapterForSummary(summary)
	if !ok {
		return nil
	}
	return runtimeProfilesForSystem(adapter.OriginSystemSlug)
}

func nativePortRuntimeProfileCompatible(summary saveSummary, profile string) bool {
	profile = strings.TrimSpace(profile)
	systemSlug := canonicalSegment(saveSummarySystemSlug(summary), "")
	if storedProfile, ok := storedOriginNativePortProfile(summary); ok {
		if strings.EqualFold(storedProfile, profile) {
			return true
		}
		adapter, adapterOK := nativePortProjectionAdapterForProfile(storedProfile)
		return adapterOK && adapter.originRuntimeProfileCompatible(profile)
	}
	if systemSlug != "" && systemSlug != nativePortSystemSlug {
		adapter, ok := nativePortProjectionAdapterForOriginSummary(summary)
		return ok && strings.EqualFold(adapter.RuntimeProfile, profile)
	}
	if systemSlug != nativePortSystemSlug {
		return false
	}
	adapter, ok := nativePortProjectionAdapterForSummary(summary)
	if !ok {
		return false
	}
	if strings.EqualFold(adapter.RuntimeProfile, profile) {
		return true
	}
	return adapter.originRuntimeProfileCompatible(profile)
}

func storedOriginNativePortProfile(summary saveSummary) (string, bool) {
	systemSlug := canonicalSegment(saveSummarySystemSlug(summary), "")
	if systemSlug == "" || systemSlug == nativePortSystemSlug {
		return "", false
	}
	if manifest, ok := nativePortManifestForRuntimeProfile(summary.RuntimeProfile); ok && manifest.OriginSystemSlug == systemSlug {
		return manifest.RuntimeProfile, true
	}
	if manifest, ok := nativePortManifestForRuntimeProfile(nativePortRuntimeProfileFromID(summary.PortID)); ok && manifest.OriginSystemSlug == systemSlug {
		return manifest.RuntimeProfile, true
	}
	return "", false
}

func normalizeN64NativePortUpload(input saveCreateInput, requestedProfile string) (saveCreateInput, error) {
	adapter, ok := nativePortProjectionAdapterForProfile(requestedProfile)
	if !ok || adapter.OriginSystemSlug != "n64" {
		return input, fmt.Errorf("native port runtimeProfile %q cannot be converted to N64 yet", requestedProfile)
	}
	canonical, err := adapter.n64CanonicalPayloadFromPort(input)
	if err != nil {
		return input, err
	}
	capable := true
	input.Payload = canonical
	input.Filename = safeFilename(adapter.OriginDisplayTitle + "." + n64CanonicalMediaByType[adapter.CanonicalMediaType].Extension)
	input.Format = inferSaveFormat(input.Filename)
	input.MediaType = adapter.CanonicalMediaType
	input.SystemSlug = adapter.OriginSystemSlug
	input.Game.System = supportedSystemFromSlug(adapter.OriginSystemSlug)
	input.DisplayTitle = adapter.OriginDisplayTitle
	input.Game.Name = adapter.OriginDisplayTitle
	input.Game.DisplayTitle = adapter.OriginDisplayTitle
	input.ProjectionCapable = &capable
	input.SourceArtifactProfile = adapter.RuntimeProfile
	input.RuntimeProfile = adapter.RuntimeProfile
	input.PortID = firstNonEmpty(input.PortID, adapter.PortID)
	input.PortName = firstNonEmpty(input.PortName, nativePortNameForID(adapter.PortID))
	input.OriginSystemSlug = adapter.OriginSystemSlug
	input.PortSaveKind = firstNonEmpty(input.PortSaveKind, "progress")
	input.SlotID = firstNonEmpty(input.SlotID, "default")
	input.Metadata = mergeRSMMetadata(input.Metadata, "nativePort", nativePortMetadataMap(input))
	return input, nil
}

func normalizeOriginNativePortUpload(input saveCreateInput, manifest nativePortManifest) saveCreateInput {
	originTitle := strings.TrimSpace(manifest.OriginGameTitle)
	if originTitle == "" {
		originTitle = strings.TrimSpace(input.DisplayTitle)
	}
	if originTitle == "" {
		originTitle = strings.TrimSpace(manifest.Name)
	}

	input.SystemSlug = manifest.OriginSystemSlug
	input.Game.System = supportedSystemFromSlug(manifest.OriginSystemSlug)
	input.DisplayTitle = originTitle
	input.Game.Name = originTitle
	input.Game.DisplayTitle = originTitle
	input.GameSlug = canonicalSegment(originTitle, input.GameSlug)
	input.SourceArtifactProfile = manifest.RuntimeProfile
	input.RuntimeProfile = manifest.RuntimeProfile
	input.PortID = firstNonEmpty(input.PortID, manifest.ID)
	input.PortName = firstNonEmpty(input.PortName, manifest.Name)
	input.OriginSystemSlug = manifest.OriginSystemSlug
	input.PortSaveKind = firstNonEmpty(input.PortSaveKind, "progress")
	if adapter, ok := nativePortProjectionAdapterForProfile(manifest.RuntimeProfile); ok && adapter.OriginIdentity {
		capable := true
		input.ProjectionCapable = &capable
	}
	input.Metadata = mergeRSMMetadata(input.Metadata, "nativePort", nativePortMetadataMap(input))
	return input
}

func projectN64ToNativePortPayload(summary saveSummary, payload []byte, requestedProfile string) (string, string, []byte, error) {
	adapter, ok := nativePortProjectionAdapterForProfile(requestedProfile)
	if !ok || adapter.OriginSystemSlug != "n64" {
		return "", "", nil, fmt.Errorf("unsupported native port runtimeProfile %q", requestedProfile)
	}
	if summaryAdapter, ok := nativePortProjectionAdapterForN64Summary(summary); !ok || summaryAdapter.PortID != adapter.PortID {
		return "", "", nil, fmt.Errorf("%s cannot be projected to %s", firstNonEmpty(summary.DisplayTitle, summary.Filename), adapter.RuntimeProfile)
	}
	portFilename, portPayload, err := adapter.n64PortPayloadFromCanonical(summary, payload)
	if err != nil {
		return "", "", nil, err
	}
	return portFilename, "application/octet-stream", portPayload, nil
}

func projectStoredOriginNativePortPayload(summary saveSummary, payload []byte, requestedProfile string) (string, string, []byte, error) {
	profile, ok := storedOriginNativePortProfile(summary)
	if !ok {
		return "", "", nil, fmt.Errorf("native port runtimeProfile %q is not compatible with this save", requestedProfile)
	}
	if strings.EqualFold(profile, requestedProfile) {
		filename := firstNonEmpty(summary.Filename, nativePortProjectionFilenameForProfile(profile))
		return filename, "application/octet-stream", append([]byte(nil), payload...), nil
	}
	adapter, adapterOK := nativePortProjectionAdapterForProfile(profile)
	if !adapterOK || !adapter.originRuntimeProfileCompatible(requestedProfile) {
		return "", "", nil, fmt.Errorf("native port runtimeProfile %q is not compatible with this save", requestedProfile)
	}
	if adapter.OriginIdentity {
		return projectIdentityRuntimePayload(summary, payload, requestedProfile)
	}
	return "", "", nil, fmt.Errorf("native port runtimeProfile %q is not compatible with %s", requestedProfile, adapter.PortID)
}

func projectOriginToNativePortPayload(summary saveSummary, payload []byte, requestedProfile string) (string, string, []byte, error) {
	adapter, ok := nativePortProjectionAdapterForProfile(requestedProfile)
	if !ok {
		return "", "", nil, fmt.Errorf("unsupported native port runtimeProfile %q", requestedProfile)
	}
	if summaryAdapter, ok := nativePortProjectionAdapterForOriginSummary(summary); !ok || summaryAdapter.PortID != adapter.PortID {
		return "", "", nil, fmt.Errorf("%s cannot be projected to %s", firstNonEmpty(summary.DisplayTitle, summary.Filename), adapter.RuntimeProfile)
	}
	if adapter.OriginIdentity {
		if adapter.ValidatePortSave != nil {
			if err := adapter.ValidatePortSave(payload); err != nil {
				return "", "", nil, err
			}
		}
		return adapter.TargetFilename, "application/octet-stream", append([]byte(nil), payload...), nil
	}
	return "", "", nil, fmt.Errorf("native port runtimeProfile %q is not compatible with %s", requestedProfile, adapter.PortID)
}

func projectNativePortPayload(summary saveSummary, payload []byte, requestedProfile string) (string, string, []byte, error) {
	adapter, ok := nativePortProjectionAdapterForSummary(summary)
	if !ok {
		return "", "", nil, fmt.Errorf("native port save does not have a safe emulator conversion adapter")
	}
	profile := strings.TrimSpace(requestedProfile)
	if strings.EqualFold(profile, adapter.RuntimeProfile) {
		return firstNonEmpty(summary.Filename, adapter.TargetFilename), "application/octet-stream", append([]byte(nil), payload...), nil
	}
	if adapter.OriginSystemSlug == "n64" && canonicalN64Profile(profile) != "" {
		canonical, err := adapter.n64CanonicalPayloadFromPort(saveCreateInput{
			Filename:       summary.Filename,
			Payload:        payload,
			RuntimeProfile: summary.RuntimeProfile,
			PortID:         summary.PortID,
			SlotID:         summary.SlotID,
			RelativePath:   summary.RelativePath,
		})
		if err != nil {
			return "", "", nil, err
		}
		originSummary := summary
		originSummary.SystemSlug = adapter.OriginSystemSlug
		originSummary.Game.System = supportedSystemFromSlug(adapter.OriginSystemSlug)
		originSummary.DisplayTitle = adapter.OriginDisplayTitle
		originSummary.Game.Name = adapter.OriginDisplayTitle
		originSummary.Game.DisplayTitle = adapter.OriginDisplayTitle
		originSummary.Filename = safeFilename(adapter.OriginDisplayTitle + "." + n64CanonicalMediaByType[adapter.CanonicalMediaType].Extension)
		originSummary.MediaType = adapter.CanonicalMediaType
		originSummary.FileSize = len(canonical)
		return projectN64Payload(originSummary, canonical, profile)
	}
	return "", "", nil, fmt.Errorf("native port runtimeProfile %q is not compatible with %s", requestedProfile, adapter.PortID)
}

func (adapter nativePortProjectionAdapter) n64CanonicalPayloadFromPort(input saveCreateInput) ([]byte, error) {
	if adapter.PortToN64Canonical != nil {
		return adapter.PortToN64Canonical(input)
	}
	if adapter.CanonicalMediaType != "eeprom" {
		return nil, fmt.Errorf("native port adapter %s does not support N64 %s conversion", adapter.PortID, adapter.CanonicalMediaType)
	}
	window, err := n64SmallEEPROMWindow(input.Payload, adapter.OriginDisplayTitle)
	if err != nil {
		return nil, err
	}
	if adapter.ValidatePortSave != nil {
		if err := adapter.ValidatePortSave(window); err != nil {
			return nil, fmt.Errorf("%s port save is not valid: %w", adapter.PortID, err)
		}
	}
	return normalizeN64EEPROM(window), nil
}

func (adapter nativePortProjectionAdapter) n64PortPayloadFromCanonical(summary saveSummary, payload []byte) (string, []byte, error) {
	if adapter.N64CanonicalToPort != nil {
		return adapter.N64CanonicalToPort(summary, payload)
	}
	if adapter.CanonicalMediaType != "eeprom" {
		return "", nil, fmt.Errorf("native port adapter %s does not support N64 %s conversion", adapter.PortID, adapter.CanonicalMediaType)
	}
	window, err := n64SmallEEPROMWindow(payload, adapter.OriginDisplayTitle)
	if err != nil {
		return "", nil, err
	}
	if adapter.ValidatePortSave != nil {
		if err := adapter.ValidatePortSave(window); err != nil {
			return "", nil, fmt.Errorf("%s canonical save is not valid for port projection: %w", adapter.PortID, err)
		}
	}
	return adapter.TargetFilename, window, nil
}

func (adapter nativePortProjectionAdapter) originRuntimeProfileCompatible(profile string) bool {
	if adapter.OriginSystemSlug == "n64" {
		return canonicalN64Profile(profile) != ""
	}
	if !adapter.OriginIdentity {
		return false
	}
	canonical := canonicalRuntimeProfile(adapter.OriginSystemSlug, profile)
	definition, ok := runtimeProfilesByID[canonical]
	return ok && definition.SystemSlug == adapter.OriginSystemSlug
}

func nativePortNameForID(portID string) string {
	if manifest, ok := nativePortManifests[canonicalOptionalSegment(portID)]; ok {
		return manifest.Name
	}
	return strings.TrimSpace(portID)
}

func saveSummarySystemSlug(summary saveSummary) string {
	if strings.TrimSpace(summary.SystemSlug) != "" {
		return summary.SystemSlug
	}
	if summary.Game.System != nil {
		return summary.Game.System.Slug
	}
	return ""
}

func nativePortProjectionFilenameForProfile(profile string) string {
	if adapter, ok := nativePortProjectionAdapterForProfile(profile); ok {
		return adapter.TargetFilename
	}
	if manifest, ok := nativePortManifestForRuntimeProfile(profile); ok {
		for _, pattern := range manifest.AllowedPatterns {
			if !strings.Contains(pattern, "*") {
				return filepath.Base(pattern)
			}
		}
	}
	return "default.sav"
}
