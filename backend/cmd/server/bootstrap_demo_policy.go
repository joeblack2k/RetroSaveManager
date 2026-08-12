package main

import "strings"

const bootstrapSeedAppPasswordCompact = "ASDK9P"

// applyBootstrapDemoPolicy removes the in-memory example state when demo data
// is disabled. newApp constructs that state before loading persisted data, so
// the production entry point must explicitly discard any untouched bootstrap
// records instead of exposing a known helper credential on a fresh install.
func (a *app) applyBootstrapDemoPolicy() {
	if a == nil || seedBootstrapEnabled() {
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	if record, ok := a.appPasswords["app-password-1"]; ok && isBootstrapSeedAppPassword(record) {
		delete(a.appPasswords, "app-password-1")
		for deviceID, current := range a.devices {
			if current.BoundAppPasswordID == nil || *current.BoundAppPasswordID != "app-password-1" {
				continue
			}
			current.BoundAppPasswordID = nil
			current.BoundAppPasswordName = ""
			a.devices[deviceID] = current
		}
	}

	if current, ok := a.devices[1]; ok && isBootstrapSeedDevice(current) {
		delete(a.devices, 1)
	}
	if current, ok := a.trustedDevices["trusted-1"]; ok && isBootstrapTrustedDevice(current) {
		delete(a.trustedDevices, "trusted-1")
	}

	delete(a.library, "lib-1")
	delete(a.catalog, "cat-1")
	delete(a.catalog, "cat-2")
	delete(a.roadmapItems, "roadmap-1")
	delete(a.roadmapItems, "roadmap-2")

	if len(a.devices) == 0 && a.nextDeviceID == 2 {
		a.nextDeviceID = 1
	}
	if len(a.appPasswords) == 0 && a.nextAppPasswordID == 2 {
		a.nextAppPasswordID = 1
	}
	if len(a.library) == 0 && a.nextLibraryID == 2 {
		a.nextLibraryID = 1
	}
	if len(a.roadmapSuggestions) == 0 && a.nextSuggestionID == 2 {
		a.nextSuggestionID = 1
	}
}

func isBootstrapSeedAppPassword(record appPassword) bool {
	return strings.TrimSpace(record.ID) == "app-password-1" &&
		strings.EqualFold(strings.TrimSpace(record.Name), "default") &&
		verifyAppPasswordCompact(record, bootstrapSeedAppPasswordCompact)
}

func isBootstrapSeedDevice(record device) bool {
	return record.ID == 1 &&
		strings.EqualFold(strings.TrimSpace(record.DeviceType), "internal") &&
		strings.EqualFold(strings.TrimSpace(record.Fingerprint), "seed0001")
}

func isBootstrapTrustedDevice(record trustedDevice) bool {
	return strings.TrimSpace(record.ID) == "trusted-1" &&
		strings.EqualFold(strings.TrimSpace(record.Name), "internal seed0001")
}
