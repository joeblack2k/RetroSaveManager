package main

import (
	"encoding/json"
	"time"
)

type loginRequest struct {
	Email       string `json:"email"`
	Password    string `json:"password"`
	DeviceType  string `json:"deviceType"`
	Fingerprint string `json:"fingerprint"`
}

type tokenResponse struct {
	Success       bool   `json:"success"`
	Token         string `json:"token"`
	ExpiresInDays int    `json:"expiresInDays"`
}

type apiError struct {
	Error      string `json:"error"`
	Message    string `json:"message"`
	Reason     string `json:"reason,omitempty"`
	StatusCode int    `json:"statusCode"`
}

type lookupQuery struct {
	Type  string          `json:"type"`
	Value json.RawMessage `json:"value"`
}

type gamesLookupRequest struct {
	Items []lookupQuery `json:"items"`
}

type saveBatchItem struct {
	Filename string          `json:"filename"`
	Game     json.RawMessage `json:"game"`
	Data     string          `json:"data"`
}

type saveBatchUploadRequest struct {
	Items []saveBatchItem `json:"items"`
}

type devicePatchRequest struct {
	Alias              *string               `json:"alias"`
	SyncAll            *bool                 `json:"syncAll"`
	AllowedSystemSlugs *[]string             `json:"allowedSystemSlugs"`
	ConfigSources      *[]deviceConfigSource `json:"configSources"`
	ConfigGlobal       *deviceConfigGlobal   `json:"configGlobal"`
}

type deviceConfigReportRequest struct {
	DeviceType          string               `json:"deviceType"`
	DeviceTypeSnake     string               `json:"device_type"`
	Fingerprint         string               `json:"fingerprint"`
	AppPassword         string               `json:"appPassword"`
	AppPasswordSnake    string               `json:"app_password"`
	Hostname            string               `json:"hostname"`
	HelperName          string               `json:"helperName"`
	HelperNameSnake     string               `json:"helper_name"`
	HelperVersion       string               `json:"helperVersion"`
	HelperVersionSnake  string               `json:"helper_version"`
	Platform            string               `json:"platform"`
	SyncPaths           []string             `json:"syncPaths"`
	SyncPathsSnake      []string             `json:"sync_paths"`
	Systems             []string             `json:"systems"`
	ReportedSystemSlugs []string             `json:"reportedSystemSlugs"`
	ConfigRevision      string               `json:"configRevision"`
	Sources             []deviceConfigSource `json:"sources"`
}

type helperConfigSyncRequest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Helper        helperConfigSyncHelper `json:"helper"`
	Config        helperConfigSyncConfig `json:"config"`
	Capabilities  map[string]any         `json:"capabilities,omitempty"`
}

type helperConfigSyncHelper struct {
	Name        string `json:"name"`
	Version     string `json:"version"`
	DeviceType  string `json:"deviceType"`
	Fingerprint string `json:"fingerprint,omitempty"`
	DefaultKind string `json:"defaultKind"`
	Hostname    string `json:"hostname"`
	Platform    string `json:"platform"`
	Arch        string `json:"arch"`
	ConfigPath  string `json:"configPath"`
	BinaryDir   string `json:"binaryDir"`
}

type helperConfigSyncConfig struct {
	URL                   string               `json:"url"`
	Port                  int                  `json:"port"`
	BaseURL               string               `json:"baseUrl"`
	Email                 string               `json:"email"`
	AppPasswordConfigured bool                 `json:"appPasswordConfigured"`
	Root                  string               `json:"root"`
	StateDir              string               `json:"stateDir"`
	Watch                 bool                 `json:"watch"`
	WatchInterval         int                  `json:"watchInterval"`
	ForceUpload           bool                 `json:"forceUpload"`
	DryRun                bool                 `json:"dryRun"`
	RoutePrefix           string               `json:"routePrefix"`
	Sources               []deviceConfigSource `json:"sources"`
}

type helperHeartbeatRequest struct {
	SchemaVersion int                    `json:"schemaVersion"`
	Helper        helperHeartbeatHelper  `json:"helper"`
	Service       helperHeartbeatService `json:"service"`
	Sensors       helperHeartbeatSensors `json:"sensors"`
	Config        helperConfigSyncConfig `json:"config"`
	Capabilities  map[string]any         `json:"capabilities,omitempty"`
}

type helperHeartbeatHelper struct {
	Name          string     `json:"name"`
	Version       string     `json:"version"`
	DeviceType    string     `json:"deviceType"`
	Fingerprint   string     `json:"fingerprint,omitempty"`
	DefaultKind   string     `json:"defaultKind"`
	Hostname      string     `json:"hostname"`
	Platform      string     `json:"platform"`
	Arch          string     `json:"arch"`
	PID           int        `json:"pid"`
	StartedAt     *time.Time `json:"startedAt"`
	UptimeSeconds int64      `json:"uptimeSeconds"`
	BinaryPath    string     `json:"binaryPath"`
	BinaryDir     string     `json:"binaryDir"`
	ConfigPath    string     `json:"configPath"`
	StateDir      string     `json:"stateDir"`
}

type helperHeartbeatService struct {
	Mode               string     `json:"mode"`
	Status             string     `json:"status"`
	Loop               string     `json:"loop"`
	HeartbeatInterval  int        `json:"heartbeatInterval"`
	ReconcileInterval  int        `json:"reconcileInterval"`
	ControlChannel     string     `json:"controlChannel"`
	LastSyncStartedAt  *time.Time `json:"lastSyncStartedAt"`
	LastSyncFinishedAt *time.Time `json:"lastSyncFinishedAt"`
	LastSyncOk         *bool      `json:"lastSyncOk"`
	LastError          *string    `json:"lastError"`
	LastEvent          string     `json:"lastEvent"`
	SyncCycles         int        `json:"syncCycles"`
}

type helperHeartbeatSensors struct {
	Online            bool                 `json:"online"`
	Authenticated     bool                 `json:"authenticated"`
	ConfigHash        string               `json:"configHash"`
	ConfigReadable    bool                 `json:"configReadable"`
	ConfigError       *string              `json:"configError"`
	SourceCount       int                  `json:"sourceCount"`
	SavePathCount     int                  `json:"savePathCount"`
	ROMPathCount      int                  `json:"romPathCount"`
	ConfiguredSystems []string             `json:"configuredSystems"`
	SupportedSystems  []string             `json:"supportedSystems"`
	SyncLockPresent   bool                 `json:"syncLockPresent"`
	LastSync          *deviceLastSyncStats `json:"lastSync"`
}

type deviceCommandRequest struct {
	Command string `json:"command"`
	Reason  string `json:"reason"`
}
