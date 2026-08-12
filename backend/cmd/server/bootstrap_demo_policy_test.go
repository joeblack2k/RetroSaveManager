package main

import "testing"

func TestApplyBootstrapDemoPolicyRemovesSeedStateWhenDisabled(t *testing.T) {
	t.Setenv("STATE_ROOT", t.TempDir())
	t.Setenv("BOOTSTRAP_DEMO_DATA", "false")

	a := newApp()
	if _, ok := a.appPasswords["app-password-1"]; !ok {
		t.Fatal("test precondition failed: bootstrap app password is missing")
	}

	a.applyBootstrapDemoPolicy()

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.appPasswords["app-password-1"]; ok {
		t.Fatal("known bootstrap app password remains enabled when demo data is disabled")
	}
	if _, ok := a.devices[1]; ok {
		t.Fatal("bootstrap device remains present when demo data is disabled")
	}
	if _, ok := a.trustedDevices["trusted-1"]; ok {
		t.Fatal("bootstrap trusted device remains present when demo data is disabled")
	}
	if len(a.catalog) != 0 {
		t.Fatalf("catalog contains %d bootstrap records, want 0", len(a.catalog))
	}
	if len(a.library) != 0 {
		t.Fatalf("library contains %d bootstrap records, want 0", len(a.library))
	}
	if len(a.roadmapItems) != 0 {
		t.Fatalf("roadmap contains %d bootstrap records, want 0", len(a.roadmapItems))
	}
	if a.nextDeviceID != 1 {
		t.Fatalf("nextDeviceID = %d, want 1", a.nextDeviceID)
	}
	if a.nextAppPasswordID != 1 {
		t.Fatalf("nextAppPasswordID = %d, want 1", a.nextAppPasswordID)
	}
	if a.nextLibraryID != 1 {
		t.Fatalf("nextLibraryID = %d, want 1", a.nextLibraryID)
	}
	if a.nextSuggestionID != 1 {
		t.Fatalf("nextSuggestionID = %d, want 1", a.nextSuggestionID)
	}
}

func TestApplyBootstrapDemoPolicyKeepsSeedStateWhenEnabled(t *testing.T) {
	t.Setenv("STATE_ROOT", t.TempDir())
	t.Setenv("BOOTSTRAP_DEMO_DATA", "true")

	a := newApp()
	a.applyBootstrapDemoPolicy()

	a.mu.Lock()
	defer a.mu.Unlock()

	if _, ok := a.appPasswords["app-password-1"]; !ok {
		t.Fatal("bootstrap app password was removed while demo data is enabled")
	}
	if _, ok := a.devices[1]; !ok {
		t.Fatal("bootstrap device was removed while demo data is enabled")
	}
	if _, ok := a.trustedDevices["trusted-1"]; !ok {
		t.Fatal("bootstrap trusted device was removed while demo data is enabled")
	}
	if len(a.catalog) != 2 {
		t.Fatalf("catalog contains %d records, want 2", len(a.catalog))
	}
	if len(a.library) != 1 {
		t.Fatalf("library contains %d records, want 1", len(a.library))
	}
	if len(a.roadmapItems) != 2 {
		t.Fatalf("roadmap contains %d records, want 2", len(a.roadmapItems))
	}
}

func TestApplyBootstrapDemoPolicyPreservesNonSeedState(t *testing.T) {
	t.Setenv("STATE_ROOT", t.TempDir())
	t.Setenv("BOOTSTRAP_DEMO_DATA", "false")

	a := newApp()
	a.mu.Lock()
	a.devices[42] = device{ID: 42, DeviceType: "android", Fingerprint: "phone-42"}
	a.appPasswords["custom"] = appPassword{
		ID:      "custom",
		Name:    "custom",
		KeySalt: "salt",
		KeyHash: hashAppPasswordCompact("salt", "ABC234"),
	}
	a.mu.Unlock()

	a.applyBootstrapDemoPolicy()

	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.devices[42]; !ok {
		t.Fatal("non-bootstrap device was removed")
	}
	if _, ok := a.appPasswords["custom"]; !ok {
		t.Fatal("non-bootstrap app password was removed")
	}
}
