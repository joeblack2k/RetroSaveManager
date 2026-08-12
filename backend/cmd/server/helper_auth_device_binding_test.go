package main

import (
	"net/http"
	"testing"
	"time"
)

// newHelperAuthTestApp returns an app with empty device / app-password state so
// each test can construct the exact binding scenario it needs without the seed
// data created by newApp(). securityStateFile is cleared so no state is persisted
// to disk during the test.
func newHelperAuthTestApp() *app {
	a := newApp()
	a.devices = map[int]device{}
	a.appPasswords = map[string]appPassword{}
	a.nextDeviceID = 1
	a.nextAppPasswordID = 1
	a.securityStateFile = ""
	return a
}

// mintHelperAppPassword creates an app-password and returns its record ID along
// with the compact form a helper would present for authentication.
func mintHelperAppPassword(t *testing.T, a *app, now time.Time) (string, string) {
	t.Helper()
	record, plain, err := a.createAppPasswordLocked("test-helper", now)
	if err != nil {
		t.Fatalf("create helper app password: %v", err)
	}
	_, compact, ok := normalizeAppPasswordInput(plain)
	if !ok {
		t.Fatalf("failed to normalize generated app password %q", plain)
	}
	return record.ID, compact
}

// An app-password that is correctly bound to the requesting device must
// authenticate cleanly (this is the regular happy-path upload).
func TestAuthenticateHelperKeyCorrectlyBoundSucceeds(t *testing.T) {
	a := newHelperAuthTestApp()
	now := time.Now().UTC()
	a.devices[1] = device{ID: 1, DeviceType: "linux-x86", Fingerprint: "deck-1", LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.nextDeviceID = 2

	keyID, compact := mintHelperAppPassword(t, a, now)
	rec := a.appPasswords[keyID]
	devID := 1
	rec.BoundDeviceID = &devID
	rec.BoundDeviceType = "linux-x86"
	rec.BoundFingerprint = "deck-1"
	a.appPasswords[keyID] = rec
	d := a.devices[1]
	d.BoundAppPasswordID = &keyID
	a.devices[1] = d

	ctx, status, msg := a.authenticateHelperKey(compact, helperIdentity{DeviceType: "linux-x86", Fingerprint: "deck-1"}, helperMetadata{})
	if status != 0 {
		t.Fatalf("expected success, got status %d (%s)", status, msg)
	}
	if ctx.AppPassword.BoundDeviceID == nil || *ctx.AppPassword.BoundDeviceID != 1 {
		t.Fatalf("expected app password bound to device 1, got %v", ctx.AppPassword.BoundDeviceID)
	}
}

// The regression case: the record stores a stale device ID (the old device no
// longer exists after a state reload) but the same physical fingerprint resolves
// to a current device. The request must rebind rather than return 409.
func TestAuthenticateHelperKeyRebindsStaleDeviceID(t *testing.T) {
	a := newHelperAuthTestApp()
	now := time.Now().UTC()
	// The live device is ID 7 (e.g. re-created with a new ID after a restart).
	a.devices[7] = device{ID: 7, DeviceType: "linux-x86", Fingerprint: "deck-1", LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.nextDeviceID = 8

	keyID, compact := mintHelperAppPassword(t, a, now)
	rec := a.appPasswords[keyID]
	staleID := 99 // device 99 no longer exists
	rec.BoundDeviceID = &staleID
	rec.BoundDeviceType = "linux-x86"
	rec.BoundFingerprint = "deck-1"
	a.appPasswords[keyID] = rec

	ctx, status, msg := a.authenticateHelperKey(compact, helperIdentity{DeviceType: "linux-x86", Fingerprint: "deck-1"}, helperMetadata{})
	if status != 0 {
		t.Fatalf("expected rebind success, got status %d (%s)", status, msg)
	}
	if got := a.appPasswords[keyID].BoundDeviceID; got == nil || *got != 7 {
		t.Fatalf("expected app password rebound to device 7, got %v", got)
	}
	if ctx.Device.ID != 7 {
		t.Fatalf("expected context device 7, got %d", ctx.Device.ID)
	}
}

// A stale device ID that points to a device with a *different* fingerprint is a
// genuinely different machine: the 409 guard must still fire.
func TestAuthenticateHelperKeyRejectsDifferentDevice(t *testing.T) {
	a := newHelperAuthTestApp()
	now := time.Now().UTC()
	a.devices[1] = device{ID: 1, DeviceType: "linux-x86", Fingerprint: "deck-1", LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.devices[2] = device{ID: 2, DeviceType: "linux-x86", Fingerprint: "deck-2", LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.nextDeviceID = 3

	keyID, compact := mintHelperAppPassword(t, a, now)
	rec := a.appPasswords[keyID]
	otherID := 2 // points at a device with a different fingerprint
	rec.BoundDeviceID = &otherID
	rec.BoundDeviceType = "linux-x86"
	rec.BoundFingerprint = "deck-1"
	a.appPasswords[keyID] = rec

	_, status, _ := a.authenticateHelperKey(compact, helperIdentity{DeviceType: "linux-x86", Fingerprint: "deck-1"}, helperMetadata{})
	if status != http.StatusConflict {
		t.Fatalf("expected 409 conflict for a genuinely different device, got %d", status)
	}
	if got := a.appPasswords[keyID].BoundDeviceID; got == nil || *got != 2 {
		t.Fatalf("expected binding to remain on device 2 after rejection, got %v", got)
	}
}

// Rebinding a device to a new app-password must delete the password it displaces
// (now unbound and unusable) rather than leaving it to accumulate as an orphan.
func TestBindAppPasswordDeletesDisplacedOrphan(t *testing.T) {
	a := newHelperAuthTestApp()
	now := time.Now().UTC()
	a.devices[1] = device{ID: 1, DeviceType: "linux-x86", Fingerprint: "deck-1", LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.nextDeviceID = 2

	old, _, err := a.createAppPasswordLocked("old", now)
	if err != nil {
		t.Fatalf("create old app password: %v", err)
	}
	a.bindAppPasswordToDeviceLocked(old.ID, a.devices[1])
	fresh, _, err := a.createAppPasswordLocked("fresh", now)
	if err != nil {
		t.Fatalf("create fresh app password: %v", err)
	}

	a.bindAppPasswordToDeviceLocked(fresh.ID, a.devices[1])

	if _, ok := a.appPasswords[old.ID]; ok {
		t.Fatalf("expected displaced orphan %s to be deleted", old.ID)
	}
	if got := a.appPasswords[fresh.ID].BoundDeviceID; got == nil || *got != 1 {
		t.Fatalf("expected fresh password bound to device 1, got %v", got)
	}
	if got := a.devices[1].BoundAppPasswordID; got == nil || *got != fresh.ID {
		t.Fatalf("expected device 1 to reference fresh password, got %v", got)
	}
}

// A displaced password that is still referenced by another device must NOT be
// deleted by the orphan sweep.
func TestBindAppPasswordKeepsPasswordReferencedElsewhere(t *testing.T) {
	a := newHelperAuthTestApp()
	now := time.Now().UTC()
	shared, _, err := a.createAppPasswordLocked("shared", now)
	if err != nil {
		t.Fatalf("create shared app password: %v", err)
	}
	sharedID := shared.ID
	a.devices[1] = device{ID: 1, DeviceType: "linux-x86", Fingerprint: "deck-1", BoundAppPasswordID: &sharedID, LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.devices[2] = device{ID: 2, DeviceType: "linux-x86", Fingerprint: "deck-2", BoundAppPasswordID: &sharedID, LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.nextDeviceID = 3

	fresh, _, err := a.createAppPasswordLocked("fresh", now)
	if err != nil {
		t.Fatalf("create fresh app password: %v", err)
	}
	a.bindAppPasswordToDeviceLocked(fresh.ID, a.devices[1])

	if _, ok := a.appPasswords[sharedID]; !ok {
		t.Fatalf("password still referenced by device 2 must not be deleted")
	}
}

// A helper may present a different device_type while keeping a stable fingerprint
// (e.g. the SGM Steam Deck helper enrolls as "steamdeck" but uploads saves tagged
// with the source emulator type "retroarch"). The fingerprint is the device
// identity, so this must authenticate and stay bound to the same device.
func TestAuthenticateHelperKeyDifferentDeviceTypeSameFingerprintSucceeds(t *testing.T) {
	a := newHelperAuthTestApp()
	now := time.Now().UTC()
	a.devices[1] = device{ID: 1, DeviceType: "steamdeck", Fingerprint: "deck-1", LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.nextDeviceID = 2

	keyID, compact := mintHelperAppPassword(t, a, now)
	rec := a.appPasswords[keyID]
	devID := 1
	rec.BoundDeviceID = &devID
	rec.BoundDeviceType = "steamdeck"
	rec.BoundFingerprint = "deck-1"
	a.appPasswords[keyID] = rec
	d := a.devices[1]
	d.BoundAppPasswordID = &keyID
	a.devices[1] = d

	ctx, status, msg := a.authenticateHelperKey(compact, helperIdentity{DeviceType: "retroarch", Fingerprint: "deck-1"}, helperMetadata{})
	if status != 0 {
		t.Fatalf("expected success on same-fingerprint different-device_type, got %d (%s)", status, msg)
	}
	if ctx.AppPassword.BoundDeviceID == nil || *ctx.AppPassword.BoundDeviceID != 1 {
		t.Fatalf("expected app password to stay bound to device 1, got %v", ctx.AppPassword.BoundDeviceID)
	}
}

// A genuinely different fingerprint is a different physical device and must still
// be rejected — an app-password is bound to one device.
func TestAuthenticateHelperKeyDifferentFingerprintRejected(t *testing.T) {
	a := newHelperAuthTestApp()
	now := time.Now().UTC()
	a.devices[1] = device{ID: 1, DeviceType: "steamdeck", Fingerprint: "deck-1", LastSeenAt: now, SyncAll: true, CreatedAt: now}
	a.nextDeviceID = 2

	keyID, compact := mintHelperAppPassword(t, a, now)
	rec := a.appPasswords[keyID]
	devID := 1
	rec.BoundDeviceID = &devID
	rec.BoundDeviceType = "steamdeck"
	rec.BoundFingerprint = "deck-1"
	a.appPasswords[keyID] = rec

	if _, status, _ := a.authenticateHelperKey(compact, helperIdentity{DeviceType: "steamdeck", Fingerprint: "other-deck"}, helperMetadata{}); status != http.StatusConflict {
		t.Fatalf("expected 409 on different fingerprint, got %d", status)
	}
}
