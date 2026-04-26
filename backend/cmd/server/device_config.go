package main

import (
	"encoding/json"
	"fmt"
	"sort"
	"strconv"
	"strings"
	"time"
)

var misterFPGAAllowedSystems = []string{
	"atari-lynx",
	"colecovision",
	"game-gear",
	"gameboy",
	"gba",
	"genesis",
	"master-system",
	"n64",
	"neogeo",
	"nes",
	"pc-engine",
	"psx",
	"saturn",
	"sega-32x",
	"sega-cd",
	"sg-1000",
	"snes",
	"wonderswan",
}

func normalizeDeviceConfigGlobal(input *deviceConfigGlobal) *deviceConfigGlobal {
	if input == nil {
		return nil
	}
	out := *input
	out.URL = strings.TrimSpace(out.URL)
	out.BaseURL = strings.TrimSpace(out.BaseURL)
	out.Email = strings.TrimSpace(out.Email)
	out.Root = strings.TrimSpace(out.Root)
	out.StateDir = strings.TrimSpace(out.StateDir)
	out.RoutePrefix = strings.TrimSpace(out.RoutePrefix)
	if out.Port < 0 {
		out.Port = 0
	}
	if out.WatchInterval < 0 {
		out.WatchInterval = 0
	}
	if out.URL == "" &&
		out.Port == 0 &&
		out.BaseURL == "" &&
		out.Email == "" &&
		!out.AppPasswordConfigured &&
		out.Root == "" &&
		out.StateDir == "" &&
		!out.Watch &&
		out.WatchInterval == 0 &&
		!out.ForceUpload &&
		!out.DryRun &&
		out.RoutePrefix == "" {
		return nil
	}
	return &out
}

func cloneConfigCapabilities(input map[string]any) map[string]any {
	if len(input) == 0 {
		return nil
	}
	data, err := json.Marshal(input)
	if err != nil {
		return nil
	}
	var out map[string]any
	if err := json.Unmarshal(data, &out); err != nil {
		return nil
	}
	return out
}

func cloneDeviceServiceState(input *deviceServiceState) *deviceServiceState {
	if input == nil {
		return nil
	}
	out := *input
	out.StartedAt = copyTimePtr(input.StartedAt)
	out.LastSyncStartedAt = copyTimePtr(input.LastSyncStartedAt)
	out.LastSyncFinishedAt = copyTimePtr(input.LastSyncFinishedAt)
	out.LastHeartbeatAt = copyTimePtr(input.LastHeartbeatAt)
	out.OfflineAt = copyTimePtr(input.OfflineAt)
	if input.LastSyncOk != nil {
		value := *input.LastSyncOk
		out.LastSyncOk = &value
	}
	return &out
}

func cloneDeviceSensorState(input *deviceSensorState) *deviceSensorState {
	if input == nil {
		return nil
	}
	out := *input
	out.ConfiguredSystems = append([]string(nil), normalizeAllowedSystemSlugs(input.ConfiguredSystems)...)
	out.SupportedSystems = append([]string(nil), normalizeAllowedSystemSlugs(input.SupportedSystems)...)
	if input.LastSync != nil {
		stats := *input.LastSync
		out.LastSync = &stats
	}
	return &out
}

func computeDeviceServiceStatus(input *deviceServiceState, now time.Time) *deviceServiceState {
	if input == nil {
		return nil
	}
	out := cloneDeviceServiceState(input)
	if out == nil {
		return nil
	}
	heartbeatInterval := out.HeartbeatInterval
	if heartbeatInterval <= 0 {
		heartbeatInterval = 30
	}
	staleAfter := maxInt(heartbeatInterval*3, 90)
	offlineAfter := maxInt(heartbeatInterval*6, 180)
	out.StaleAfterSeconds = staleAfter
	out.OfflineAfterSeconds = offlineAfter
	out.Freshness = "offline"
	out.Online = false

	if out.LastHeartbeatAt == nil {
		return out
	}
	if strings.EqualFold(strings.TrimSpace(out.Status), "stopping") {
		out.Freshness = "offline"
		offlineAt := out.LastHeartbeatAt.UTC()
		out.OfflineAt = &offlineAt
		return out
	}
	age := now.Sub(out.LastHeartbeatAt.UTC())
	switch {
	case age > time.Duration(offlineAfter)*time.Second:
		out.Freshness = "offline"
		offlineAt := out.LastHeartbeatAt.Add(time.Duration(offlineAfter) * time.Second).UTC()
		out.OfflineAt = &offlineAt
	case age > time.Duration(staleAfter)*time.Second:
		out.Freshness = "stale"
		out.Online = true
	default:
		out.Online = true
		out.Freshness = "online"
		if strings.EqualFold(strings.TrimSpace(out.Status), "backoff") || strings.TrimSpace(out.LastError) != "" || (out.LastSyncOk != nil && !*out.LastSyncOk) {
			out.Freshness = "degraded"
		}
	}
	return out
}

func (s *deviceConfigSource) UnmarshalJSON(data []byte) error {
	var raw map[string]any
	if err := json.Unmarshal(data, &raw); err != nil {
		return err
	}

	out := deviceConfigSource{}
	out.ID = firstNonEmpty(stringFromAny(raw["id"]), stringFromAny(raw["ID"]), stringFromAny(raw["name"]), stringFromAny(raw["Name"]), stringFromAny(raw["NAME"]))
	out.Label = firstNonEmpty(stringFromAny(raw["label"]), stringFromAny(raw["LABEL"]))
	out.Kind = firstNonEmpty(stringFromAny(raw["kind"]), stringFromAny(raw["KIND"]))
	out.Profile = firstNonEmpty(stringFromAny(raw["profile"]), stringFromAny(raw["PROFILE"]))
	out.SavePath = firstNonEmpty(stringFromAny(raw["savePath"]), stringFromAny(raw["save_path"]), stringFromAny(raw["SAVE_PATH"]))
	if values := stringListFromAny(firstNonEmptyValue(raw["savePaths"], raw["save_roots"])); len(values) > 0 {
		out.SavePaths = values
	}
	out.ROMPath = firstNonEmpty(stringFromAny(raw["romPath"]), stringFromAny(raw["rom_path"]), stringFromAny(raw["ROM_PATH"]))
	if values := stringListFromAny(firstNonEmptyValue(raw["romPaths"], raw["rom_roots"])); len(values) > 0 {
		out.ROMPaths = values
	}
	out.Recursive = boolFromAny(firstNonEmptyValue(raw["recursive"], raw["RECURSIVE"]))
	out.CreateMissingSystemDirs = boolFromAny(firstNonEmptyValue(raw["createMissingSystemDirs"], raw["create_missing_system_dirs"], raw["CREATE_MISSING_SYSTEM_DIRS"]))
	out.Managed = boolFromAny(firstNonEmptyValue(raw["managed"], raw["MANAGED"]))
	out.Origin = firstNonEmpty(stringFromAny(raw["origin"]), stringFromAny(raw["ORIGIN"]))
	if values := stringListFromAny(firstNonEmptyValue(raw["systems"], raw["SYSTEMS"])); len(values) > 0 {
		out.Systems = values
	}
	if values := stringListFromAny(firstNonEmptyValue(raw["unsupportedSystemSlugs"], raw["unsupported_system_slugs"])); len(values) > 0 {
		out.UnsupportedSystemSlugs = values
	}
	*s = normalizeDeviceConfigSource(out, 0)
	return nil
}

func normalizeDeviceConfigSources(raw []deviceConfigSource) []deviceConfigSource {
	out := make([]deviceConfigSource, 0, len(raw))
	for index, source := range raw {
		normalized := normalizeDeviceConfigSource(source, index)
		if normalized.ID == "" && normalized.Label == "" && normalized.Kind == "" && normalized.Profile == "" && normalized.SavePath == "" && normalized.ROMPath == "" && len(normalized.Systems) == 0 && len(normalized.UnsupportedSystemSlugs) == 0 {
			continue
		}
		out = append(out, normalized)
	}
	return out
}

func mergeHelperReportedConfigSources(reported []deviceConfigSource, existing []deviceConfigSource) []deviceConfigSource {
	out := normalizeDeviceConfigSources(reported)
	indexByID := make(map[string]int, len(out))
	for index, source := range out {
		indexByID[source.ID] = index
	}
	for _, source := range normalizeDeviceConfigSources(existing) {
		if !isBackendPolicySource(source) {
			continue
		}
		if index, ok := indexByID[source.ID]; ok {
			out[index] = source
			continue
		}
		indexByID[source.ID] = len(out)
		out = append(out, source)
	}
	return normalizeDeviceConfigSources(out)
}

func markBackendPolicySources(raw []deviceConfigSource) []deviceConfigSource {
	out := normalizeDeviceConfigSources(raw)
	for index := range out {
		out[index].Origin = "backend"
		out[index].Managed = true
	}
	return out
}

func isBackendPolicySource(source deviceConfigSource) bool {
	origin := canonicalOptionalSegment(source.Origin)
	return origin == "backend" || origin == "backend-ui" || origin == "server"
}

func normalizeDeviceConfigSource(input deviceConfigSource, index int) deviceConfigSource {
	out := input
	out.ID = strings.TrimSpace(out.ID)
	out.Label = strings.TrimSpace(out.Label)
	out.Kind = canonicalOptionalSegment(out.Kind)
	out.Profile = canonicalOptionalSegment(out.Profile)
	out.SavePath = strings.TrimSpace(out.SavePath)
	out.SavePaths = normalizeHelperPaths(out.SavePaths)
	if out.SavePath != "" {
		out.SavePaths = normalizeHelperPaths(append([]string{out.SavePath}, out.SavePaths...))
	}
	if len(out.SavePaths) > 0 {
		out.SavePath = out.SavePaths[0]
	}
	out.ROMPath = strings.TrimSpace(out.ROMPath)
	out.ROMPaths = normalizeHelperPaths(out.ROMPaths)
	if out.ROMPath != "" {
		out.ROMPaths = normalizeHelperPaths(append([]string{out.ROMPath}, out.ROMPaths...))
	}
	if len(out.ROMPaths) > 0 {
		out.ROMPath = out.ROMPaths[0]
	}
	out.Origin = canonicalOptionalSegment(out.Origin)
	rawSystems := append([]string(nil), out.Systems...)
	rawSystems = append(rawSystems, out.UnsupportedSystemSlugs...)
	out.Systems, out.UnsupportedSystemSlugs = normalizeConfigSystemSlugs(rawSystems)
	if out.ID == "" {
		base := firstNonEmpty(out.Label, out.Kind+"-"+out.Profile, out.SavePath, fmt.Sprintf("source-%d", index+1))
		out.ID = canonicalSegment(base, fmt.Sprintf("source-%d", index+1))
	}
	if out.Label == "" {
		out.Label = toDisplayWords(out.ID)
	}
	return out
}

func normalizeConfigSystemSlugs(raw []string) ([]string, []string) {
	acceptedSeen := map[string]struct{}{}
	rejectedSeen := map[string]struct{}{}
	accepted := make([]string, 0, len(raw))
	rejected := make([]string, 0)
	for _, item := range raw {
		value := strings.TrimSpace(item)
		if value == "" {
			continue
		}
		normalized := supportedSystemSlugFromLabel(value)
		if normalized != "" && isSupportedSystemSlug(normalized) {
			if _, exists := acceptedSeen[normalized]; !exists {
				acceptedSeen[normalized] = struct{}{}
				accepted = append(accepted, normalized)
			}
			continue
		}
		unsupported := canonicalSegment(value, "")
		if unsupported == "" || unsupported == "unknown-system" {
			continue
		}
		if _, exists := rejectedSeen[unsupported]; exists {
			continue
		}
		rejectedSeen[unsupported] = struct{}{}
		rejected = append(rejected, unsupported)
	}
	sort.Strings(accepted)
	sort.Strings(rejected)
	return accepted, rejected
}

func effectiveDevicePolicy(deviceInput device) deviceEffectivePolicy {
	sources := normalizeDeviceConfigSources(deviceInput.ConfigSources)
	if len(sources) == 0 {
		if deviceInput.SyncAll {
			return deviceEffectivePolicy{
				Mode:               "legacy-all-supported",
				AllowedSystemSlugs: allSupportedSystemSlugs(),
			}
		}
		return deviceEffectivePolicy{
			Mode:               "manual-allow-list",
			AllowedSystemSlugs: normalizeAllowedSystemSlugs(deviceInput.AllowedSystemSlugs),
		}
	}

	sourcePolicies := make([]deviceSourceEffectivePolicy, 0, len(sources))
	sourceAllowedSet := map[string]struct{}{}
	blocked := make([]devicePolicyBlock, 0)
	for _, source := range sources {
		policy := effectiveSourcePolicy(deviceInput.DeviceType, source)
		for _, slug := range policy.AllowedSystemSlugs {
			sourceAllowedSet[slug] = struct{}{}
		}
		blocked = append(blocked, policy.Blocked...)
		sourcePolicies = append(sourcePolicies, policy)
	}

	sourceAllowed := sortedKeys(sourceAllowedSet)
	if deviceInput.SyncAll {
		return deviceEffectivePolicy{
			Mode:               "source-scoped-all",
			AllowedSystemSlugs: sourceAllowed,
			Blocked:            blocked,
			Sources:            sourcePolicies,
		}
	}

	manualAllowed := normalizeAllowedSystemSlugs(deviceInput.AllowedSystemSlugs)
	allowed := intersectSorted(manualAllowed, sourceAllowed)
	sourceAllowedLookup := stringSet(sourceAllowed)
	for _, slug := range manualAllowed {
		if _, ok := sourceAllowedLookup[slug]; !ok {
			blocked = append(blocked, devicePolicyBlock{
				System: slug,
				Reason: "not reported by helper config or blocked by source capability",
			})
		}
	}

	return deviceEffectivePolicy{
		Mode:               "manual-source-intersection",
		AllowedSystemSlugs: allowed,
		Blocked:            sortDevicePolicyBlocks(blocked),
		Sources:            sourcePolicies,
	}
}

func effectiveSourcePolicy(deviceType string, source deviceConfigSource) deviceSourceEffectivePolicy {
	allowed := append([]string(nil), source.Systems...)
	blocked := make([]devicePolicyBlock, 0, len(source.UnsupportedSystemSlugs))
	for _, slug := range source.UnsupportedSystemSlugs {
		blocked = append(blocked, devicePolicyBlock{
			System:      slug,
			Reason:      "unsupported by backend",
			SourceID:    source.ID,
			SourceLabel: source.Label,
		})
	}

	if capability, constrained := sourceCapabilitySet(deviceType, source); constrained {
		filtered := make([]string, 0, len(allowed))
		for _, slug := range allowed {
			if _, ok := capability[slug]; ok {
				filtered = append(filtered, slug)
				continue
			}
			blocked = append(blocked, devicePolicyBlock{
				System:      slug,
				Reason:      "not supported by this helper kind/profile",
				SourceID:    source.ID,
				SourceLabel: source.Label,
			})
		}
		allowed = filtered
	}

	sort.Strings(allowed)
	return deviceSourceEffectivePolicy{
		SourceID:           source.ID,
		SourceLabel:        source.Label,
		Kind:               source.Kind,
		Profile:            source.Profile,
		AllowedSystemSlugs: allowed,
		Blocked:            sortDevicePolicyBlocks(blocked),
	}
}

func sourceCapabilitySet(deviceType string, source deviceConfigSource) (map[string]struct{}, bool) {
	kind := canonicalOptionalSegment(source.Kind)
	profile := canonicalOptionalSegment(source.Profile)
	deviceKind := canonicalOptionalSegment(deviceType)
	keys := []string{
		strings.Trim(kind+"/"+profile, "/"),
		kind,
		profile,
		deviceKind,
	}
	for _, key := range keys {
		switch key {
		case "mister-fpga/mister", "mister/mister", "mister-fpga", "mister":
			return stringSet(misterFPGAAllowedSystems), true
		}
	}
	return nil, false
}

func systemAllowedForDevice(deviceInput device, systemSlug string) bool {
	normalizedSystem := canonicalSegment(systemSlug, "unknown-system")
	if !isSupportedSystemSlug(normalizedSystem) {
		return false
	}
	for _, slug := range effectiveDevicePolicy(deviceInput).AllowedSystemSlugs {
		if slug == normalizedSystem {
			return true
		}
	}
	return false
}

func allSupportedSystemSlugs() []string {
	systems := allSupportedSystems()
	out := make([]string, 0, len(systems))
	for _, sys := range systems {
		if slug := canonicalOptionalSegment(sys.Slug); slug != "" {
			out = append(out, slug)
		}
	}
	sort.Strings(out)
	return out
}

func deriveConfigSyncPaths(sources []deviceConfigSource) []string {
	out := make([]string, 0, len(sources)*2)
	for _, source := range sources {
		out = append(out, source.SavePaths...)
		out = append(out, source.ROMPaths...)
	}
	return normalizeHelperPaths(out)
}

func deriveConfigReportedSystems(sources []deviceConfigSource) []string {
	seen := map[string]struct{}{}
	for _, source := range sources {
		for _, slug := range source.Systems {
			seen[slug] = struct{}{}
		}
	}
	return sortedKeys(seen)
}

func stringSet(values []string) map[string]struct{} {
	out := make(map[string]struct{}, len(values))
	for _, value := range values {
		if value = canonicalOptionalSegment(value); value != "" {
			out[value] = struct{}{}
		}
	}
	return out
}

func sortedKeys(values map[string]struct{}) []string {
	out := make([]string, 0, len(values))
	for value := range values {
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func intersectSorted(left, right []string) []string {
	rightSet := stringSet(right)
	out := make([]string, 0, len(left))
	seen := map[string]struct{}{}
	for _, value := range left {
		value = canonicalOptionalSegment(value)
		if value == "" {
			continue
		}
		if _, ok := rightSet[value]; !ok {
			continue
		}
		if _, exists := seen[value]; exists {
			continue
		}
		seen[value] = struct{}{}
		out = append(out, value)
	}
	sort.Strings(out)
	return out
}

func sortDevicePolicyBlocks(blocks []devicePolicyBlock) []devicePolicyBlock {
	out := append([]devicePolicyBlock(nil), blocks...)
	sort.Slice(out, func(i, j int) bool {
		if out[i].System == out[j].System {
			if out[i].SourceID == out[j].SourceID {
				return out[i].Reason < out[j].Reason
			}
			return out[i].SourceID < out[j].SourceID
		}
		return out[i].System < out[j].System
	})
	return out
}

func firstNonEmptyValue(values ...any) any {
	for _, value := range values {
		switch typed := value.(type) {
		case nil:
			continue
		case string:
			if strings.TrimSpace(typed) == "" {
				continue
			}
		case []any:
			if len(typed) == 0 {
				continue
			}
		case []string:
			if len(typed) == 0 {
				continue
			}
		}
		return value
	}
	return nil
}

func firstNonEmptyBool(current bool, values ...any) any {
	if current {
		return true
	}
	return firstNonEmptyValue(values...)
}

func stringFromAny(value any) string {
	switch typed := value.(type) {
	case string:
		return strings.TrimSpace(typed)
	case fmt.Stringer:
		return strings.TrimSpace(typed.String())
	case float64:
		return strconv.FormatFloat(typed, 'f', -1, 64)
	case bool:
		if typed {
			return "true"
		}
		return "false"
	default:
		return ""
	}
}

func boolFromAny(value any) bool {
	switch typed := value.(type) {
	case bool:
		return typed
	case string:
		normalized := strings.ToLower(strings.TrimSpace(typed))
		return normalized == "true" || normalized == "1" || normalized == "yes" || normalized == "on"
	case float64:
		return typed != 0
	case int:
		return typed != 0
	default:
		return false
	}
}

func stringListFromAny(value any) []string {
	switch typed := value.(type) {
	case []string:
		return append([]string(nil), typed...)
	case []any:
		out := make([]string, 0, len(typed))
		for _, item := range typed {
			if value := stringFromAny(item); value != "" {
				out = append(out, value)
			}
		}
		return out
	case string:
		return splitHelperListField(typed)
	default:
		return nil
	}
}
