package main

import "testing"

func armedHelperConfig() helperConfigSyncConfig {
	return helperConfigSyncConfig{
		URL: "saves-sync.example", Port: 443, StateDir: "state",
		DryRun: true, ForceUpload: true,
	}
}

// The two builders turn a helper's report into the config the server stores.
// Dry-run and force-upload are policy, not device facts: if the builders copy
// them, one --dry-run pass becomes the device's permanent policy.
func TestHelperConfigBuildersIgnoreReportedPolicy(t *testing.T) {
	sync := helperConfigGlobal(helperConfigSyncRequest{Config: armedHelperConfig()})
	if sync == nil {
		t.Fatal("sync builder returned nil")
	}
	if sync.DryRun || sync.ForceUpload {
		t.Fatalf("sync builder copied reported policy: dryRun=%v forceUpload=%v", sync.DryRun, sync.ForceUpload)
	}

	hb := helperHeartbeatConfigGlobal(helperHeartbeatRequest{Config: armedHelperConfig()})
	if hb == nil {
		t.Fatal("heartbeat builder returned nil")
	}
	if hb.DryRun || hb.ForceUpload {
		t.Fatalf("heartbeat builder copied reported policy: dryRun=%v forceUpload=%v", hb.DryRun, hb.ForceUpload)
	}
}

// A fresh device whose first run is --dry-run must not come out armed. This is
// the case a guard on the *existing* config cannot catch: there is no existing
// config on the first report.
func TestFreshDeviceSelfReportMustNotArmPolicy(t *testing.T) {
	a := &app{devices: map[int]device{}}

	_ = a.storeDeviceConfigSnapshotLocked(device{ID: 9, Fingerprint: "fresh"}, deviceConfigSnapshot{
		ConfigRevision: "rev-1",
		ConfigGlobal:   helperConfigGlobal(helperConfigSyncRequest{Config: armedHelperConfig()}),
	})

	stored := a.devices[9].ConfigGlobal
	if stored == nil {
		t.Fatal("expected stored config, got nil")
	}
	if stored.DryRun {
		t.Fatal("fresh device: helper self-report armed dry-run with no operator config present")
	}
}

// The operator path writes ConfigGlobal directly; this guards that the helper
// fix did not quietly take that path with it.
func TestOperatorConfigKeepsDryRun(t *testing.T) {
	got := normalizeDeviceConfigGlobal(&deviceConfigGlobal{DryRun: true})
	if got == nil || !got.DryRun {
		t.Fatal("an operator-set dry-run must survive normalisation")
	}
}
