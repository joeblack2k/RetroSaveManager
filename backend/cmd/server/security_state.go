package main

import (
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"time"
)

const (
	defaultStateRoot            = "./data/state"
	securityDeviceStateFileName = "security_device_state.json"
	appPasswordAlphabet         = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"
)

type securityDeviceStateFile struct {
	NextDeviceID                int                        `json:"nextDeviceId"`
	NextAppPasswordID           int                        `json:"nextAppPasswordId"`
	AutoAppPasswordEnabledUntil *time.Time                 `json:"autoAppPasswordEnabledUntil,omitempty"`
	Devices                     []securityStateDevice      `json:"devices"`
	AppPasswords                []securityStateAppPassword `json:"appPasswords"`
	UpdatedAt                   time.Time                  `json:"updatedAt"`
}

type securityStateDevice struct {
	ID                  int       `json:"id"`
	DeviceType          string    `json:"deviceType"`
	Fingerprint         string    `json:"fingerprint"`
	Alias               *string   `json:"alias"`
	DisplayName         string    `json:"displayName"`
	Hostname            string    `json:"hostname,omitempty"`
	HelperName          string    `json:"helperName,omitempty"`
	HelperVersion       string    `json:"helperVersion,omitempty"`
	Platform            string    `json:"platform,omitempty"`
	SyncPaths           []string  `json:"syncPaths,omitempty"`
	ReportedSystemSlugs []string  `json:"reportedSystemSlugs,omitempty"`
	LastSeenIP          string    `json:"lastSeenIp,omitempty"`
	LastSeenUserAgent   string    `json:"lastSeenUserAgent,omitempty"`
	LastSeenAt          time.Time `json:"lastSeenAt"`
	SyncAll             *bool     `json:"syncAll,omitempty"`
	AllowedSystemSlugs  []string  `json:"allowedSystemSlugs,omitempty"`
	BoundAppPasswordID  *string   `json:"boundAppPasswordId,omitempty"`
	LastSyncedAt        time.Time `json:"lastSyncedAt"`
	CreatedAt           time.Time `json:"createdAt"`
}

type securityStateAppPassword struct {
	ID               string     `json:"id"`
	Name             string     `json:"name"`
	LastFour         string     `json:"lastFour"`
	CreatedAt        time.Time  `json:"createdAt"`
	LastUsed         *time.Time `json:"lastUsedAt,omitempty"`
	BoundDeviceID    *int       `json:"boundDeviceId,omitempty"`
	BoundDeviceType  string     `json:"boundDeviceType,omitempty"`
	BoundFingerprint string     `json:"boundFingerprint,omitempty"`
	KeySalt          string     `json:"keySalt"`
	KeyHash          string     `json:"keyHash"`
}

func stateRootDirFromEnv() string {
	root := strings.TrimSpace(os.Getenv("STATE_ROOT"))
	if root == "" {
		root = defaultStateRoot
	}
	if absRoot, err := filepath.Abs(root); err == nil {
		root = absRoot
	}
	_ = os.MkdirAll(root, 0o755)
	return root
}

func securityDeviceStateFilePathFromEnv() string {
	return filepath.Join(stateRootDirFromEnv(), securityDeviceStateFileName)
}

func defaultDeviceDisplayName(deviceType, fingerprint string) string {
	typeLabel := strings.TrimSpace(deviceType)
	fingerprintLabel := strings.TrimSpace(fingerprint)
	if typeLabel == "" && fingerprintLabel == "" {
		return "Unknown device"
	}
	if typeLabel == "" {
		return fingerprintLabel
	}
	if fingerprintLabel == "" {
		return typeLabel
	}
	return typeLabel + " " + fingerprintLabel
}

func normalizeAllowedSystemSlugs(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		normalized := canonicalSegment(item, "")
		if normalized == "" || normalized == "unknown-system" || !isSupportedSystemSlug(normalized) {
			continue
		}
		if _, exists := seen[normalized]; exists {
			continue
		}
		seen[normalized] = struct{}{}
		out = append(out, normalized)
	}
	sort.Strings(out)
	if len(out) == 0 {
		return nil
	}
	return out
}

func normalizeHelperPaths(raw []string) []string {
	seen := make(map[string]struct{}, len(raw))
	out := make([]string, 0, len(raw))
	for _, item := range raw {
		trimmed := strings.TrimSpace(item)
		if trimmed == "" {
			continue
		}
		if _, exists := seen[trimmed]; exists {
			continue
		}
		seen[trimmed] = struct{}{}
		out = append(out, trimmed)
	}
	if len(out) == 0 {
		return nil
	}
	return out
}

func formatAppPasswordCompact(compact string) string {
	clean := strings.ToUpper(strings.TrimSpace(compact))
	if len(clean) != 6 {
		return clean
	}
	return clean[:3] + "-" + clean[3:]
}

func normalizeAppPasswordInput(raw string) (formatted string, compact string, ok bool) {
	value := strings.ToUpper(strings.TrimSpace(raw))
	value = strings.ReplaceAll(value, " ", "")
	value = strings.ReplaceAll(value, "-", "")
	if len(value) != 6 {
		return "", "", false
	}
	for _, r := range value {
		if (r < 'A' || r > 'Z') && (r < '0' || r > '9') {
			return "", "", false
		}
	}
	return formatAppPasswordCompact(value), value, true
}

func generateAppPasswordCompact() string {
	buf := make([]byte, 6)
	if _, err := rand.Read(buf); err != nil {
		return "ASDK9P"
	}
	out := make([]byte, 6)
	for i, value := range buf {
		out[i] = appPasswordAlphabet[int(value)%len(appPasswordAlphabet)]
	}
	return string(out)
}

func hashAppPasswordCompact(salt, compact string) string {
	h := sha256.Sum256([]byte(strings.TrimSpace(salt) + ":" + strings.TrimSpace(strings.ToUpper(compact))))
	return hex.EncodeToString(h[:])
}

func verifyAppPasswordCompact(record appPassword, compact string) bool {
	if strings.TrimSpace(record.KeySalt) == "" || strings.TrimSpace(record.KeyHash) == "" {
		return false
	}
	hash := hashAppPasswordCompact(record.KeySalt, compact)
	return subtle.ConstantTimeCompare([]byte(hash), []byte(record.KeyHash)) == 1
}

func copyStringPtr(value *string) *string {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func copyIntPtr(value *int) *int {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func copyBoolPtr(value *bool) *bool {
	if value == nil {
		return nil
	}
	copyValue := *value
	return &copyValue
}

func copyTimePtr(value *time.Time) *time.Time {
	if value == nil {
		return nil
	}
	copyValue := value.UTC()
	return &copyValue
}

func (a *app) loadSecurityDeviceState() error {
	if a == nil || strings.TrimSpace(a.securityStateFile) == "" {
		return nil
	}

	data, err := os.ReadFile(a.securityStateFile)
	if err != nil {
		if os.IsNotExist(err) {
			return nil
		}
		return fmt.Errorf("read security state: %w", err)
	}
	if len(data) == 0 {
		return nil
	}

	var file securityDeviceStateFile
	if err := json.Unmarshal(data, &file); err != nil {
		return fmt.Errorf("decode security state: %w", err)
	}

	devices := make(map[int]device, len(file.Devices))
	maxDeviceID := 0
	for _, persisted := range file.Devices {
		if persisted.ID <= 0 {
			continue
		}
		syncAll := true
		if persisted.SyncAll != nil {
			syncAll = *persisted.SyncAll
		}
		d := device{
			ID:                  persisted.ID,
			DeviceType:          strings.TrimSpace(persisted.DeviceType),
			Fingerprint:         strings.TrimSpace(persisted.Fingerprint),
			Alias:               copyStringPtr(persisted.Alias),
			DisplayName:         strings.TrimSpace(persisted.DisplayName),
			Hostname:            strings.TrimSpace(persisted.Hostname),
			HelperName:          strings.TrimSpace(persisted.HelperName),
			HelperVersion:       strings.TrimSpace(persisted.HelperVersion),
			Platform:            strings.TrimSpace(persisted.Platform),
			SyncPaths:           normalizeHelperPaths(persisted.SyncPaths),
			ReportedSystemSlugs: normalizeAllowedSystemSlugs(persisted.ReportedSystemSlugs),
			LastSeenIP:          strings.TrimSpace(persisted.LastSeenIP),
			LastSeenUserAgent:   strings.TrimSpace(persisted.LastSeenUserAgent),
			LastSeenAt:          persisted.LastSeenAt,
			SyncAll:             syncAll,
			AllowedSystemSlugs:  normalizeAllowedSystemSlugs(persisted.AllowedSystemSlugs),
			BoundAppPasswordID:  copyStringPtr(persisted.BoundAppPasswordID),
			LastSyncedAt:        persisted.LastSyncedAt,
			CreatedAt:           persisted.CreatedAt,
		}
		if d.LastSeenAt.IsZero() && !d.LastSyncedAt.IsZero() {
			d.LastSeenAt = d.LastSyncedAt
		}
		if d.DisplayName == "" {
			d.DisplayName = defaultDeviceDisplayName(d.DeviceType, d.Fingerprint)
		}
		devices[d.ID] = d
		if d.ID > maxDeviceID {
			maxDeviceID = d.ID
		}
	}

	passwords := make(map[string]appPassword, len(file.AppPasswords))
	maxPasswordID := 0
	for _, persisted := range file.AppPasswords {
		id := strings.TrimSpace(persisted.ID)
		if id == "" {
			continue
		}
		record := appPassword{
			ID:                 id,
			Name:               strings.TrimSpace(persisted.Name),
			LastFour:           strings.TrimSpace(persisted.LastFour),
			CreatedAt:          persisted.CreatedAt,
			LastUsed:           persisted.LastUsed,
			BoundDeviceID:      copyIntPtr(persisted.BoundDeviceID),
			SyncAll:            true,
			AllowedSystemSlugs: nil,
			KeySalt:            strings.TrimSpace(persisted.KeySalt),
			KeyHash:            strings.TrimSpace(persisted.KeyHash),
			BoundDeviceType:    strings.TrimSpace(persisted.BoundDeviceType),
			BoundFingerprint:   strings.TrimSpace(persisted.BoundFingerprint),
		}
		if record.Name == "" {
			record.Name = "app-password"
		}
		if record.KeySalt == "" || record.KeyHash == "" {
			continue
		}
		passwords[record.ID] = record
		if parsed := parseIntOrDefault(strings.TrimPrefix(record.ID, "app-password-"), 0); parsed > maxPasswordID {
			maxPasswordID = parsed
		}
	}

	a.devices = devices
	a.appPasswords = passwords

	if file.NextDeviceID > 0 {
		a.nextDeviceID = file.NextDeviceID
	} else {
		a.nextDeviceID = maxDeviceID + 1
	}
	if a.nextDeviceID <= 0 {
		a.nextDeviceID = 1
	}

	if file.NextAppPasswordID > 0 {
		a.nextAppPasswordID = file.NextAppPasswordID
	} else {
		a.nextAppPasswordID = maxPasswordID + 1
	}
	if a.nextAppPasswordID <= 0 {
		a.nextAppPasswordID = 1
	}
	if file.AutoAppPasswordEnabledUntil != nil {
		copyUntil := file.AutoAppPasswordEnabledUntil.UTC()
		a.autoAppPasswordEnabledUntil = &copyUntil
	} else {
		a.autoAppPasswordEnabledUntil = nil
	}

	for id, d := range a.devices {
		a.devices[id] = a.publicDeviceLocked(d)
	}

	return nil
}

func (a *app) securityDeviceStateSnapshotLocked() securityDeviceStateFile {
	deviceIDs := make([]int, 0, len(a.devices))
	for id := range a.devices {
		deviceIDs = append(deviceIDs, id)
	}
	sort.Ints(deviceIDs)
	persistedDevices := make([]securityStateDevice, 0, len(deviceIDs))
	for _, id := range deviceIDs {
		d := a.devices[id]
		syncAll := d.SyncAll
		persistedDevices = append(persistedDevices, securityStateDevice{
			ID:                  d.ID,
			DeviceType:          d.DeviceType,
			Fingerprint:         d.Fingerprint,
			Alias:               copyStringPtr(d.Alias),
			DisplayName:         d.DisplayName,
			Hostname:            strings.TrimSpace(d.Hostname),
			HelperName:          strings.TrimSpace(d.HelperName),
			HelperVersion:       strings.TrimSpace(d.HelperVersion),
			Platform:            strings.TrimSpace(d.Platform),
			SyncPaths:           append([]string(nil), normalizeHelperPaths(d.SyncPaths)...),
			ReportedSystemSlugs: append([]string(nil), normalizeAllowedSystemSlugs(d.ReportedSystemSlugs)...),
			LastSeenIP:          strings.TrimSpace(d.LastSeenIP),
			LastSeenUserAgent:   strings.TrimSpace(d.LastSeenUserAgent),
			LastSeenAt:          d.LastSeenAt,
			SyncAll:             copyBoolPtr(&syncAll),
			AllowedSystemSlugs:  append([]string(nil), d.AllowedSystemSlugs...),
			BoundAppPasswordID:  copyStringPtr(d.BoundAppPasswordID),
			LastSyncedAt:        d.LastSyncedAt,
			CreatedAt:           d.CreatedAt,
		})
	}

	passwordIDs := make([]string, 0, len(a.appPasswords))
	for id := range a.appPasswords {
		passwordIDs = append(passwordIDs, id)
	}
	sort.Strings(passwordIDs)
	persistedPasswords := make([]securityStateAppPassword, 0, len(passwordIDs))
	for _, id := range passwordIDs {
		record := a.appPasswords[id]
		persistedPasswords = append(persistedPasswords, securityStateAppPassword{
			ID:               record.ID,
			Name:             record.Name,
			LastFour:         record.LastFour,
			CreatedAt:        record.CreatedAt,
			LastUsed:         record.LastUsed,
			BoundDeviceID:    copyIntPtr(record.BoundDeviceID),
			BoundDeviceType:  record.BoundDeviceType,
			BoundFingerprint: record.BoundFingerprint,
			KeySalt:          record.KeySalt,
			KeyHash:          record.KeyHash,
		})
	}

	return securityDeviceStateFile{
		NextDeviceID:                a.nextDeviceID,
		NextAppPasswordID:           a.nextAppPasswordID,
		AutoAppPasswordEnabledUntil: copyTimePtr(a.autoAppPasswordEnabledUntil),
		Devices:                     persistedDevices,
		AppPasswords:                persistedPasswords,
		UpdatedAt:                   time.Now().UTC(),
	}
}

func (a *app) persistSecurityDeviceStateLocked() error {
	if a == nil || strings.TrimSpace(a.securityStateFile) == "" {
		return nil
	}
	snapshot := a.securityDeviceStateSnapshotLocked()
	data, err := json.MarshalIndent(snapshot, "", "  ")
	if err != nil {
		return fmt.Errorf("encode security state: %w", err)
	}
	if err := writeFileAtomic(a.securityStateFile, data, 0o644); err != nil {
		return fmt.Errorf("write security state: %w", err)
	}
	return nil
}

func (a *app) publicDeviceLocked(input device) device {
	out := input
	if out.DisplayName == "" {
		out.DisplayName = defaultDeviceDisplayName(out.DeviceType, out.Fingerprint)
	}
	out.Hostname = strings.TrimSpace(out.Hostname)
	out.HelperName = strings.TrimSpace(out.HelperName)
	out.HelperVersion = strings.TrimSpace(out.HelperVersion)
	out.Platform = strings.TrimSpace(out.Platform)
	out.SyncPaths = normalizeHelperPaths(out.SyncPaths)
	out.ReportedSystemSlugs = normalizeAllowedSystemSlugs(out.ReportedSystemSlugs)
	out.LastSeenIP = strings.TrimSpace(out.LastSeenIP)
	out.LastSeenUserAgent = strings.TrimSpace(out.LastSeenUserAgent)
	if out.LastSeenAt.IsZero() && !out.LastSyncedAt.IsZero() {
		out.LastSeenAt = out.LastSyncedAt
	}
	out.AllowedSystemSlugs = normalizeAllowedSystemSlugs(out.AllowedSystemSlugs)
	out.BoundAppPasswordName = ""
	out.BoundAppPasswordLastFour = ""
	if out.BoundAppPasswordID != nil {
		if password, ok := a.appPasswords[*out.BoundAppPasswordID]; ok {
			out.BoundAppPasswordName = password.Name
			out.BoundAppPasswordLastFour = password.LastFour
		}
	}
	return out
}

func (a *app) publicAppPasswordLocked(input appPassword) appPassword {
	out := input
	out.SyncAll = true
	out.AllowedSystemSlugs = nil
	if out.BoundDeviceID != nil {
		if boundDevice, ok := a.devices[*out.BoundDeviceID]; ok {
			out.SyncAll = boundDevice.SyncAll
			out.AllowedSystemSlugs = append([]string(nil), normalizeAllowedSystemSlugs(boundDevice.AllowedSystemSlugs)...)
		} else {
			out.BoundDeviceID = nil
		}
	}
	return out
}

func (a *app) findAppPasswordByCompactLocked(compact string) (appPassword, bool) {
	for _, candidate := range a.appPasswords {
		if verifyAppPasswordCompact(candidate, compact) {
			return candidate, true
		}
	}
	return appPassword{}, false
}

func (a *app) appPasswordIDForDeviceLocked(deviceID int) (string, bool) {
	for _, record := range a.appPasswords {
		if record.BoundDeviceID != nil && *record.BoundDeviceID == deviceID {
			return record.ID, true
		}
	}
	return "", false
}

func (a *app) autoAppPasswordWindowActiveLocked(now time.Time) bool {
	if a == nil || a.autoAppPasswordEnabledUntil == nil {
		return false
	}
	return now.UTC().Before(a.autoAppPasswordEnabledUntil.UTC())
}

func (a *app) enableAutoAppPasswordWindowLocked(duration time.Duration) time.Time {
	if duration <= 0 {
		duration = 15 * time.Minute
	}
	until := time.Now().UTC().Add(duration)
	a.autoAppPasswordEnabledUntil = &until
	return until
}

func (a *app) generateUniqueAppPasswordCompactLocked() (string, bool) {
	for attempt := 0; attempt < 64; attempt++ {
		candidateCompact := generateAppPasswordCompact()
		exists := false
		for _, existing := range a.appPasswords {
			if verifyAppPasswordCompact(existing, candidateCompact) {
				exists = true
				break
			}
		}
		if !exists {
			return candidateCompact, true
		}
	}
	return "", false
}

func (a *app) createAppPasswordLocked(name string, now time.Time) (appPassword, string) {
	trimmedName := strings.TrimSpace(name)
	if trimmedName == "" {
		trimmedName = "app-password"
	}

	id := a.nextAppPasswordID
	a.nextAppPasswordID++

	compact, ok := a.generateUniqueAppPasswordCompactLocked()
	if !ok {
		compact = "ASDK9P"
	}
	salt := randomHex(16)
	record := appPassword{
		ID:                 "app-password-" + strconv.Itoa(id),
		Name:               trimmedName,
		LastFour:           compact[len(compact)-4:],
		CreatedAt:          now.UTC(),
		BoundDeviceID:      nil,
		SyncAll:            true,
		AllowedSystemSlugs: nil,
		KeySalt:            salt,
		KeyHash:            hashAppPasswordCompact(salt, compact),
	}
	a.appPasswords[record.ID] = record
	return record, formatAppPasswordCompact(compact)
}

func (a *app) bindAppPasswordToDeviceLocked(passwordID string, d device) {
	record, ok := a.appPasswords[passwordID]
	if !ok {
		return
	}

	if current := d.BoundAppPasswordID; current != nil && *current != "" && *current != passwordID {
		if previous, exists := a.appPasswords[*current]; exists {
			previous.BoundDeviceID = nil
			previous.BoundDeviceType = ""
			previous.BoundFingerprint = ""
			a.appPasswords[*current] = previous
		}
	}

	for deviceID, candidate := range a.devices {
		if candidate.BoundAppPasswordID == nil || *candidate.BoundAppPasswordID != passwordID || deviceID == d.ID {
			continue
		}
		candidate.BoundAppPasswordID = nil
		candidate.BoundAppPasswordName = ""
		candidate.BoundAppPasswordLastFour = ""
		a.saveDeviceLocked(candidate)
	}

	now := time.Now().UTC()
	record.BoundDeviceID = &d.ID
	record.BoundDeviceType = strings.TrimSpace(d.DeviceType)
	record.BoundFingerprint = strings.TrimSpace(d.Fingerprint)
	record.LastUsed = &now
	a.appPasswords[passwordID] = record

	passwordIDCopy := passwordID
	d.BoundAppPasswordID = &passwordIDCopy
	d.LastSyncedAt = now
	if d.LastSeenAt.IsZero() || d.LastSeenAt.Before(now) {
		d.LastSeenAt = now
	}
	a.saveDeviceLocked(d)
}

func applyHelperMetadataToDevice(input device, metadata helperMetadata, seenAt time.Time) device {
	out := input
	if seenAt.IsZero() {
		seenAt = time.Now().UTC()
	}
	out.LastSeenAt = seenAt.UTC()

	if hostname := strings.TrimSpace(metadata.Hostname); hostname != "" {
		out.Hostname = hostname
	}
	if helperName := strings.TrimSpace(metadata.HelperName); helperName != "" {
		out.HelperName = helperName
	}
	if helperVersion := strings.TrimSpace(metadata.HelperVersion); helperVersion != "" {
		out.HelperVersion = helperVersion
	}
	if platform := strings.TrimSpace(metadata.Platform); platform != "" {
		out.Platform = platform
	}
	if len(metadata.SyncPaths) > 0 {
		out.SyncPaths = normalizeHelperPaths(metadata.SyncPaths)
	}
	if len(metadata.ReportedSystemSlugs) > 0 {
		out.ReportedSystemSlugs = normalizeAllowedSystemSlugs(metadata.ReportedSystemSlugs)
	}
	if ip := strings.TrimSpace(metadata.LastSeenIP); ip != "" {
		out.LastSeenIP = ip
	}
	if userAgent := strings.TrimSpace(metadata.LastSeenUserAgent); userAgent != "" {
		out.LastSeenUserAgent = userAgent
	}
	return out
}

func deviceIdentityMatches(deviceTypeA, fingerprintA, deviceTypeB, fingerprintB string) bool {
	return strings.EqualFold(strings.TrimSpace(deviceTypeA), strings.TrimSpace(deviceTypeB)) && strings.EqualFold(strings.TrimSpace(fingerprintA), strings.TrimSpace(fingerprintB))
}

func (a *app) findDeviceByIdentityLocked(deviceType, fingerprint string) (device, bool) {
	for _, d := range a.devices {
		if deviceIdentityMatches(d.DeviceType, d.Fingerprint, deviceType, fingerprint) {
			return d, true
		}
	}
	return device{}, false
}

func (a *app) upsertDeviceLocked(deviceType, fingerprint string) device {
	now := time.Now().UTC()
	for id, d := range a.devices {
		if deviceIdentityMatches(d.DeviceType, d.Fingerprint, deviceType, fingerprint) {
			d.LastSeenAt = now
			d.LastSyncedAt = now
			a.devices[id] = a.publicDeviceLocked(d)
			return a.devices[id]
		}
	}

	deviceID := a.nextDeviceID
	a.nextDeviceID++
	created := device{
		ID:                 deviceID,
		DeviceType:         strings.TrimSpace(deviceType),
		Fingerprint:        strings.TrimSpace(fingerprint),
		Alias:              nil,
		DisplayName:        defaultDeviceDisplayName(deviceType, fingerprint),
		LastSeenAt:         now,
		SyncAll:            true,
		AllowedSystemSlugs: nil,
		LastSyncedAt:       now,
		CreatedAt:          now,
	}
	a.devices[deviceID] = created
	return created
}

func (a *app) saveDeviceLocked(input device) {
	a.devices[input.ID] = a.publicDeviceLocked(input)
}

func systemAllowedForDevice(deviceInput device, systemSlug string) bool {
	normalizedSystem := canonicalSegment(systemSlug, "unknown-system")
	if !isSupportedSystemSlug(normalizedSystem) {
		return false
	}
	if deviceInput.SyncAll {
		return true
	}
	allowed := normalizeAllowedSystemSlugs(deviceInput.AllowedSystemSlugs)
	if len(allowed) == 0 {
		return false
	}
	for _, slug := range allowed {
		if slug == normalizedSystem {
			return true
		}
	}
	return false
}
