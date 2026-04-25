package main

import (
	"encoding/json"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/go-chi/chi/v5"
)

func (a *app) handleDevicesList(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	a.mu.Lock()
	defer a.mu.Unlock()

	devices := make([]device, 0, len(a.devices))
	for _, d := range a.devices {
		devices = append(devices, a.publicDeviceLocked(d))
	}
	sort.Slice(devices, func(i, j int) bool { return devices[i].ID > devices[j].ID })

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "devices": devices})
}

func (a *app) handleDevicesGet(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	id := parseIntOrDefault(chi.URLParam(r, "id"), 0)
	a.mu.Lock()

	d, ok := a.devices[id]
	if !ok {
		a.mu.Unlock()
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Device not found", StatusCode: http.StatusNotFound})
		return
	}
	public := a.publicDeviceLocked(d)
	a.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "device": public})
}

func (a *app) handleDevicesPatch(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	id := parseIntOrDefault(chi.URLParam(r, "id"), 0)
	var req devicePatchRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	a.mu.Lock()

	d, ok := a.devices[id]
	if !ok {
		a.mu.Unlock()
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Device not found", StatusCode: http.StatusNotFound})
		return
	}

	if req.Alias != nil {
		alias := strings.TrimSpace(*req.Alias)
		if alias == "" {
			d.Alias = nil
			d.DisplayName = defaultDeviceDisplayName(d.DeviceType, d.Fingerprint)
		} else {
			d.Alias = &alias
			d.DisplayName = alias
		}
	}
	if req.SyncAll != nil {
		d.SyncAll = *req.SyncAll
	}
	if req.AllowedSystemSlugs != nil {
		d.AllowedSystemSlugs = normalizeAllowedSystemSlugs(*req.AllowedSystemSlugs)
	}
	configChanged := false
	if req.ConfigGlobal != nil {
		d.ConfigGlobal = normalizeDeviceConfigGlobal(req.ConfigGlobal)
		configChanged = true
	}
	if req.ConfigSources != nil {
		d.ConfigSources = markBackendPolicySources(*req.ConfigSources)
		d.SyncPaths = deriveConfigSyncPaths(d.ConfigSources)
		d.ReportedSystemSlugs = deriveConfigReportedSystems(d.ConfigSources)
		configChanged = true
	}
	if configChanged {
		now := time.Now().UTC()
		d.ConfigReportedAt = &now
		d.ConfigRevision = "backend-policy-" + strconv.FormatInt(now.Unix(), 10)
	}

	a.saveDeviceLocked(d)
	_ = a.persistSecurityDeviceStateLocked()
	public := a.publicDeviceLocked(d)
	a.mu.Unlock()

	if configChanged {
		a.publishEvent("config.changed", map[string]any{
			"deviceId": public.ID,
			"helperId": strconv.Itoa(public.ID),
			"reason":   "device_policy_updated",
			"revision": public.ConfigRevision,
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"success": true, "device": public})
}

func (a *app) handleDevicesConfigReport(w http.ResponseWriter, r *http.Request) {
	var req deviceConfigReportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "invalid config report JSON", StatusCode: http.StatusBadRequest})
		return
	}

	sources := normalizeDeviceConfigSources(req.Sources)
	if len(sources) == 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "at least one config source is required", StatusCode: http.StatusBadRequest})
		return
	}

	helperCtx, ok := a.authorizeHelperSyncRequest(w, r, configReportFormValue(req, sources))
	if !ok {
		return
	}
	if !helperCtx.IsHelper || helperCtx.Device.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "helper identity is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	public := a.storeDeviceConfigSnapshotLocked(helperCtx.Device, deviceConfigSnapshot{
		ConfigRevision:      strings.TrimSpace(req.ConfigRevision),
		Sources:             sources,
		PreserveBackend:     true,
		SyncPaths:           append(req.SyncPaths, req.SyncPathsSnake...),
		ReportedSystemSlugs: append(req.Systems, req.ReportedSystemSlugs...),
	})
	_ = a.persistSecurityDeviceStateLocked()
	writeJSON(w, http.StatusOK, deviceConfigSyncResponse(public))
}

func (a *app) handleHelpersConfigSync(w http.ResponseWriter, r *http.Request) {
	var req helperConfigSyncRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: "invalid helper config sync JSON", StatusCode: http.StatusUnprocessableEntity})
		return
	}

	sources := normalizeDeviceConfigSources(req.Config.Sources)
	if len(sources) == 0 {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: "at least one config source is required", StatusCode: http.StatusUnprocessableEntity})
		return
	}

	helperCtx, ok := a.authorizeHelperSyncRequest(w, r, helperConfigSyncFormValue(req, sources))
	if !ok {
		return
	}
	if !helperCtx.IsHelper || helperCtx.Device.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "helper identity is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	defer a.mu.Unlock()

	public := a.storeDeviceConfigSnapshotLocked(helperCtx.Device, deviceConfigSnapshot{
		ConfigRevision:      helperConfigRevision(req),
		ConfigGlobal:        helperConfigGlobal(req),
		Sources:             sources,
		Capabilities:        req.Capabilities,
		PreserveBackend:     true,
		SyncPaths:           helperConfigSyncPaths(req),
		ReportedSystemSlugs: deriveConfigReportedSystems(sources),
		Hostname:            req.Helper.Hostname,
		HelperName:          req.Helper.Name,
		HelperVersion:       req.Helper.Version,
		Platform:            helperConfigPlatform(req),
	})
	_ = a.persistSecurityDeviceStateLocked()
	writeJSON(w, http.StatusOK, deviceConfigSyncResponse(public))
}

func (a *app) handleHelpersHeartbeat(w http.ResponseWriter, r *http.Request) {
	var req helperHeartbeatRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusUnprocessableEntity, apiError{Error: "Unprocessable Entity", Message: "invalid helper heartbeat JSON", StatusCode: http.StatusUnprocessableEntity})
		return
	}

	sources := normalizeDeviceConfigSources(req.Config.Sources)
	helperCtx, ok := a.authorizeHelperSyncRequest(w, r, helperHeartbeatFormValue(req, sources))
	if !ok {
		return
	}
	if !helperCtx.IsHelper || helperCtx.Device.ID <= 0 {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "helper identity is required", StatusCode: http.StatusBadRequest})
		return
	}

	a.mu.Lock()
	public := a.storeDeviceHeartbeatLocked(helperCtx.Device, req, sources)
	_ = a.persistSecurityDeviceStateLocked()
	a.mu.Unlock()

	writeJSON(w, http.StatusOK, map[string]any{
		"success":    true,
		"accepted":   true,
		"serverTime": time.Now().UTC(),
		"helperId":   strconv.Itoa(public.ID),
		"device":     public,
	})
}

func (a *app) handleDeviceCommand(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	id := parseIntOrDefault(chi.URLParam(r, "id"), 0)
	var req deviceCommandRequest
	_ = json.NewDecoder(r.Body).Decode(&req)

	a.mu.Lock()
	d, ok := a.devices[id]
	a.mu.Unlock()
	if !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Device not found", StatusCode: http.StatusNotFound})
		return
	}

	eventType, action := helperCommandEvent(req.Command)
	if eventType == "" {
		writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: "unsupported device command", StatusCode: http.StatusBadRequest})
		return
	}
	reason := strings.TrimSpace(req.Reason)
	if reason == "" {
		reason = "user_requested"
	}
	payload := map[string]any{
		"deviceId":    id,
		"helperId":    strconv.Itoa(id),
		"deviceType":  d.DeviceType,
		"fingerprint": d.Fingerprint,
		"reason":      reason,
		"action":      action,
	}
	a.publishEvent(eventType, payload)
	a.appendSyncLog(syncLogInput{
		DeviceName: firstNonEmpty(d.DisplayName, defaultDeviceDisplayName(d.DeviceType, d.Fingerprint)),
		Action:     "command_" + action,
		Game:       "device",
	})
	writeJSON(w, http.StatusOK, map[string]any{
		"success":   true,
		"event":     eventType,
		"action":    action,
		"broadcast": true,
	})
}

func (a *app) handleDevicesDelete(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	id := parseIntOrDefault(chi.URLParam(r, "id"), 0)
	a.mu.Lock()
	defer a.mu.Unlock()
	if _, ok := a.devices[id]; !ok {
		writeJSON(w, http.StatusNotFound, apiError{Error: "Not Found", Message: "Device not found", StatusCode: http.StatusNotFound})
		return
	}
	delete(a.devices, id)
	for passwordID, record := range a.appPasswords {
		if record.BoundDeviceID == nil || *record.BoundDeviceID != id {
			continue
		}
		record.BoundDeviceID = nil
		a.appPasswords[passwordID] = record
	}
	_ = a.persistSecurityDeviceStateLocked()
	writeJSON(w, http.StatusOK, map[string]any{"success": true})
}

type deviceConfigSnapshot struct {
	ConfigRevision      string
	ConfigGlobal        *deviceConfigGlobal
	Sources             []deviceConfigSource
	Capabilities        map[string]any
	PreserveBackend     bool
	SyncPaths           []string
	ReportedSystemSlugs []string
	Hostname            string
	HelperName          string
	HelperVersion       string
	Platform            string
}

func (a *app) storeDeviceConfigSnapshotLocked(base device, snapshot deviceConfigSnapshot) device {
	now := time.Now().UTC()
	d, exists := a.devices[base.ID]
	if !exists {
		d = base
	}
	d.ConfigRevision = strings.TrimSpace(snapshot.ConfigRevision)
	d.ConfigReportedAt = &now
	d.ConfigGlobal = normalizeDeviceConfigGlobal(snapshot.ConfigGlobal)
	if snapshot.PreserveBackend {
		d.ConfigSources = mergeHelperReportedConfigSources(snapshot.Sources, d.ConfigSources)
	} else {
		d.ConfigSources = normalizeDeviceConfigSources(snapshot.Sources)
	}
	d.ConfigCapabilities = cloneConfigCapabilities(snapshot.Capabilities)
	if hostname := strings.TrimSpace(snapshot.Hostname); hostname != "" {
		d.Hostname = hostname
	}
	if helperName := strings.TrimSpace(snapshot.HelperName); helperName != "" {
		d.HelperName = helperName
	}
	if helperVersion := strings.TrimSpace(snapshot.HelperVersion); helperVersion != "" {
		d.HelperVersion = helperVersion
	}
	if platform := strings.TrimSpace(snapshot.Platform); platform != "" {
		d.Platform = platform
	}
	if paths := normalizeHelperPaths(snapshot.SyncPaths); len(paths) > 0 {
		d.SyncPaths = paths
	} else if paths := deriveConfigSyncPaths(d.ConfigSources); len(paths) > 0 {
		d.SyncPaths = paths
	}
	if systems := normalizeAllowedSystemSlugs(snapshot.ReportedSystemSlugs); len(systems) > 0 {
		d.ReportedSystemSlugs = systems
	} else if systems := deriveConfigReportedSystems(d.ConfigSources); len(systems) > 0 {
		d.ReportedSystemSlugs = systems
	}
	d.LastSeenAt = now
	d.LastSyncedAt = now

	a.saveDeviceLocked(d)
	return a.publicDeviceLocked(a.devices[d.ID])
}

func (a *app) storeDeviceHeartbeatLocked(base device, req helperHeartbeatRequest, sources []deviceConfigSource) device {
	now := time.Now().UTC()
	d, exists := a.devices[base.ID]
	if !exists {
		d = base
	}
	d.ConfigRevision = firstNonEmpty(strings.TrimSpace(req.Sensors.ConfigHash), helperConfigRevision(helperConfigSyncRequest{SchemaVersion: req.SchemaVersion}))
	d.ConfigReportedAt = &now
	d.ConfigGlobal = helperHeartbeatConfigGlobal(req)
	if len(sources) > 0 {
		d.ConfigSources = mergeHelperReportedConfigSources(sources, d.ConfigSources)
	}
	d.ConfigCapabilities = cloneConfigCapabilities(req.Capabilities)
	if hostname := strings.TrimSpace(req.Helper.Hostname); hostname != "" {
		d.Hostname = hostname
	}
	if helperName := strings.TrimSpace(req.Helper.Name); helperName != "" {
		d.HelperName = helperName
	}
	if helperVersion := strings.TrimSpace(req.Helper.Version); helperVersion != "" {
		d.HelperVersion = helperVersion
	}
	if platform := helperHeartbeatPlatform(req); platform != "" {
		d.Platform = platform
	}
	if paths := deriveConfigSyncPaths(sources); len(paths) > 0 {
		d.SyncPaths = paths
	} else if paths := deriveConfigSyncPaths(d.ConfigSources); len(paths) > 0 {
		d.SyncPaths = paths
	}
	if systems := normalizeAllowedSystemSlugs(req.Sensors.ConfiguredSystems); len(systems) > 0 {
		d.ReportedSystemSlugs = systems
	} else if systems := deriveConfigReportedSystems(sources); len(systems) > 0 {
		d.ReportedSystemSlugs = systems
	}
	d.Service = helperHeartbeatServiceState(req, now)
	d.Sensors = helperHeartbeatSensorState(req)
	d.LastSeenAt = now
	d.LastSyncedAt = now

	a.saveDeviceLocked(d)
	return a.publicDeviceLocked(a.devices[d.ID])
}

func deviceConfigSyncResponse(public device) map[string]any {
	policy := map[string]any{
		"sources": runtimeSourcePoliciesForDevice(public),
	}
	if globalPolicy := runtimeGlobalPolicyForDevice(public); len(globalPolicy) > 0 {
		policy["global"] = globalPolicy
	}
	return map[string]any{
		"success":         true,
		"accepted":        true,
		"device":          public,
		"effectivePolicy": public.EffectivePolicy,
		"policy":          policy,
	}
}

func runtimeGlobalPolicyForDevice(public device) map[string]any {
	if public.ConfigGlobal == nil {
		return nil
	}
	global := public.ConfigGlobal
	policy := map[string]any{
		"forceUpload": global.ForceUpload,
		"dryRun":      global.DryRun,
	}
	if global.URL != "" {
		policy["url"] = global.URL
	}
	if global.Port > 0 {
		policy["port"] = global.Port
	}
	if global.BaseURL != "" {
		policy["baseUrl"] = global.BaseURL
	}
	if global.Email != "" {
		policy["email"] = global.Email
	}
	if global.Root != "" {
		policy["root"] = global.Root
	}
	if global.StateDir != "" {
		policy["stateDir"] = global.StateDir
	}
	if global.Watch {
		policy["watch"] = global.Watch
	}
	if global.WatchInterval > 0 {
		policy["watchInterval"] = global.WatchInterval
	}
	if global.RoutePrefix != "" {
		policy["routePrefix"] = global.RoutePrefix
	}
	return policy
}

func runtimeSourcePoliciesForDevice(public device) []map[string]any {
	policy := effectiveDevicePolicy(public)
	sourcesByID := make(map[string]deviceConfigSource, len(public.ConfigSources))
	for _, source := range normalizeDeviceConfigSources(public.ConfigSources) {
		sourcesByID[source.ID] = source
	}
	out := make([]map[string]any, 0, len(policy.Sources))
	for _, sourcePolicy := range policy.Sources {
		systems := intersectSorted(sourcePolicy.AllowedSystemSlugs, policy.AllowedSystemSlugs)
		source := sourcesByID[sourcePolicy.SourceID]
		item := map[string]any{
			"id":                      sourcePolicy.SourceID,
			"sourceId":                sourcePolicy.SourceID,
			"name":                    sourcePolicy.SourceLabel,
			"label":                   sourcePolicy.SourceLabel,
			"enabled":                 len(systems) > 0,
			"kind":                    firstNonEmpty(source.Kind, sourcePolicy.Kind),
			"profile":                 firstNonEmpty(source.Profile, sourcePolicy.Profile),
			"recursive":               source.Recursive,
			"systems":                 systems,
			"createMissingSystemDirs": source.CreateMissingSystemDirs,
		}
		if len(source.SavePaths) > 0 {
			item["savePaths"] = source.SavePaths
		} else if source.SavePath != "" {
			item["savePath"] = source.SavePath
		}
		if len(source.ROMPaths) > 0 {
			item["romPaths"] = source.ROMPaths
		} else if source.ROMPath != "" {
			item["romPath"] = source.ROMPath
		}
		out = append(out, item)
	}
	return out
}

func configReportFormValue(req deviceConfigReportRequest, sources []deviceConfigSource) func(string) string {
	derivedPaths := deriveConfigSyncPaths(sources)
	derivedSystems := deriveConfigReportedSystems(sources)
	return func(key string) string {
		switch key {
		case "device_type", "deviceType":
			return firstNonEmpty(req.DeviceType, req.DeviceTypeSnake)
		case "fingerprint":
			return req.Fingerprint
		case "app_password", "appPassword":
			return firstNonEmpty(req.AppPassword, req.AppPasswordSnake)
		case "hostname", "host_name", "helper_hostname", "helperHostname":
			return req.Hostname
		case "helper_name", "helperName", "client_name", "clientName":
			return firstNonEmpty(req.HelperName, req.HelperNameSnake)
		case "helper_version", "helperVersion", "client_version", "clientVersion":
			return firstNonEmpty(req.HelperVersion, req.HelperVersionSnake)
		case "platform", "os", "os_name", "osName":
			return req.Platform
		case "sync_paths", "syncPaths", "sync_path", "syncPath", "save_root", "saveRoot", "save_roots", "saveRoots":
			if values := append(req.SyncPaths, req.SyncPathsSnake...); len(values) > 0 {
				return strings.Join(values, ",")
			}
			return strings.Join(derivedPaths, ",")
		case "systems", "system_slugs", "systemSlugs", "supported_systems", "supportedSystems":
			values := append([]string(nil), req.Systems...)
			values = append(values, req.ReportedSystemSlugs...)
			if len(values) > 0 {
				return strings.Join(values, ",")
			}
			return strings.Join(derivedSystems, ",")
		default:
			return ""
		}
	}
}

func helperConfigSyncFormValue(req helperConfigSyncRequest, sources []deviceConfigSource) func(string) string {
	derivedPaths := deriveConfigSyncPaths(sources)
	derivedSystems := deriveConfigReportedSystems(sources)
	return func(key string) string {
		switch key {
		case "device_type", "deviceType":
			return firstNonEmpty(req.Helper.DeviceType, req.Helper.DefaultKind)
		case "fingerprint":
			return firstNonEmpty(req.Helper.Fingerprint, req.Helper.Hostname, req.Helper.ConfigPath)
		case "hostname", "host_name", "helper_hostname", "helperHostname":
			return req.Helper.Hostname
		case "helper_name", "helperName", "client_name", "clientName":
			return req.Helper.Name
		case "helper_version", "helperVersion", "client_version", "clientVersion":
			return req.Helper.Version
		case "platform", "os", "os_name", "osName":
			return helperConfigPlatform(req)
		case "sync_paths", "syncPaths", "sync_path", "syncPath", "save_root", "saveRoot", "save_roots", "saveRoots":
			return strings.Join(derivedPaths, ",")
		case "systems", "system_slugs", "systemSlugs", "supported_systems", "supportedSystems":
			return strings.Join(derivedSystems, ",")
		default:
			return ""
		}
	}
}

func helperHeartbeatFormValue(req helperHeartbeatRequest, sources []deviceConfigSource) func(string) string {
	derivedPaths := deriveConfigSyncPaths(sources)
	derivedSystems := deriveConfigReportedSystems(sources)
	return func(key string) string {
		switch key {
		case "device_type", "deviceType":
			return firstNonEmpty(req.Helper.DeviceType, req.Helper.DefaultKind)
		case "fingerprint":
			return firstNonEmpty(req.Helper.Fingerprint, req.Helper.Hostname, req.Helper.ConfigPath)
		case "hostname", "host_name", "helper_hostname", "helperHostname":
			return req.Helper.Hostname
		case "helper_name", "helperName", "client_name", "clientName":
			return req.Helper.Name
		case "helper_version", "helperVersion", "client_version", "clientVersion":
			return req.Helper.Version
		case "platform", "os", "os_name", "osName":
			return helperHeartbeatPlatform(req)
		case "sync_paths", "syncPaths", "sync_path", "syncPath", "save_root", "saveRoot", "save_roots", "saveRoots":
			return strings.Join(derivedPaths, ",")
		case "systems", "system_slugs", "systemSlugs", "supported_systems", "supportedSystems":
			if len(req.Sensors.ConfiguredSystems) > 0 {
				return strings.Join(req.Sensors.ConfiguredSystems, ",")
			}
			return strings.Join(derivedSystems, ",")
		default:
			return ""
		}
	}
}

func helperConfigSyncPaths(req helperConfigSyncRequest) []string {
	return deriveConfigSyncPaths(req.Config.Sources)
}

func helperConfigRevision(req helperConfigSyncRequest) string {
	if req.SchemaVersion <= 0 {
		return ""
	}
	return "schema-v" + strings.TrimSpace(strconv.Itoa(req.SchemaVersion))
}

func helperConfigPlatform(req helperConfigSyncRequest) string {
	parts := make([]string, 0, 2)
	if platform := strings.TrimSpace(req.Helper.Platform); platform != "" {
		parts = append(parts, platform)
	}
	if arch := strings.TrimSpace(req.Helper.Arch); arch != "" {
		parts = append(parts, arch)
	}
	return strings.Join(parts, "/")
}

func helperConfigGlobal(req helperConfigSyncRequest) *deviceConfigGlobal {
	return normalizeDeviceConfigGlobal(&deviceConfigGlobal{
		URL:                   req.Config.URL,
		Port:                  req.Config.Port,
		BaseURL:               req.Config.BaseURL,
		Email:                 req.Config.Email,
		AppPasswordConfigured: req.Config.AppPasswordConfigured,
		Root:                  req.Config.Root,
		StateDir:              req.Config.StateDir,
		Watch:                 req.Config.Watch,
		WatchInterval:         req.Config.WatchInterval,
		ForceUpload:           req.Config.ForceUpload,
		DryRun:                req.Config.DryRun,
		RoutePrefix:           req.Config.RoutePrefix,
	})
}

func helperHeartbeatConfigGlobal(req helperHeartbeatRequest) *deviceConfigGlobal {
	return normalizeDeviceConfigGlobal(&deviceConfigGlobal{
		URL:                   req.Config.URL,
		Port:                  req.Config.Port,
		BaseURL:               req.Config.BaseURL,
		Email:                 req.Config.Email,
		AppPasswordConfigured: req.Config.AppPasswordConfigured,
		Root:                  req.Config.Root,
		StateDir:              firstNonEmpty(req.Helper.StateDir, req.Config.StateDir),
		Watch:                 req.Config.Watch,
		WatchInterval:         req.Config.WatchInterval,
		ForceUpload:           req.Config.ForceUpload,
		DryRun:                req.Config.DryRun,
		RoutePrefix:           req.Config.RoutePrefix,
	})
}

func helperHeartbeatPlatform(req helperHeartbeatRequest) string {
	parts := make([]string, 0, 2)
	if platform := strings.TrimSpace(req.Helper.Platform); platform != "" {
		parts = append(parts, platform)
	}
	if arch := strings.TrimSpace(req.Helper.Arch); arch != "" {
		parts = append(parts, arch)
	}
	return strings.Join(parts, "/")
}

func helperHeartbeatServiceState(req helperHeartbeatRequest, now time.Time) *deviceServiceState {
	lastError := ""
	if req.Service.LastError != nil {
		lastError = strings.TrimSpace(*req.Service.LastError)
	}
	return computeDeviceServiceStatus(&deviceServiceState{
		Mode:               strings.TrimSpace(req.Service.Mode),
		Status:             strings.TrimSpace(req.Service.Status),
		Loop:               strings.TrimSpace(req.Service.Loop),
		ControlChannel:     strings.TrimSpace(req.Service.ControlChannel),
		HeartbeatInterval:  req.Service.HeartbeatInterval,
		ReconcileInterval:  req.Service.ReconcileInterval,
		PID:                req.Helper.PID,
		StartedAt:          copyTimePtr(req.Helper.StartedAt),
		UptimeSeconds:      req.Helper.UptimeSeconds,
		BinaryPath:         strings.TrimSpace(req.Helper.BinaryPath),
		LastSyncStartedAt:  copyTimePtr(req.Service.LastSyncStartedAt),
		LastSyncFinishedAt: copyTimePtr(req.Service.LastSyncFinishedAt),
		LastSyncOk:         copyBoolPtr(req.Service.LastSyncOk),
		LastError:          lastError,
		LastEvent:          strings.TrimSpace(req.Service.LastEvent),
		SyncCycles:         req.Service.SyncCycles,
		LastHeartbeatAt:    &now,
	}, now)
}

func helperHeartbeatSensorState(req helperHeartbeatRequest) *deviceSensorState {
	configError := ""
	if req.Sensors.ConfigError != nil {
		configError = strings.TrimSpace(*req.Sensors.ConfigError)
	}
	return cloneDeviceSensorState(&deviceSensorState{
		Online:            req.Sensors.Online,
		Authenticated:     req.Sensors.Authenticated,
		ConfigHash:        strings.TrimSpace(req.Sensors.ConfigHash),
		ConfigReadable:    req.Sensors.ConfigReadable,
		ConfigError:       configError,
		SourceCount:       req.Sensors.SourceCount,
		SavePathCount:     req.Sensors.SavePathCount,
		ROMPathCount:      req.Sensors.ROMPathCount,
		ConfiguredSystems: req.Sensors.ConfiguredSystems,
		SupportedSystems:  req.Sensors.SupportedSystems,
		SyncLockPresent:   req.Sensors.SyncLockPresent,
		LastSync:          req.Sensors.LastSync,
	})
}

func helperCommandEvent(command string) (string, string) {
	action := strings.TrimSpace(strings.ToLower(command))
	action = strings.ReplaceAll(action, "-", "_")
	switch action {
	case "sync", "sync_now", "sync.requested":
		return "sync.requested", "sync"
	case "scan", "scan_now", "scan.requested":
		return "scan.requested", "scan"
	case "deep_scan", "deep_scan_now", "deep.scan", "deep_scan.requested", "deep-scan.requested":
		return "deep_scan.requested", "deep_scan"
	case "config", "config_changed", "config.changed", "reload_config":
		return "config.changed", "config.changed"
	default:
		return "", ""
	}
}
