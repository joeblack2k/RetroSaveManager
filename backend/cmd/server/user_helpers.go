package main

import "net/http"

const internalPrincipalUserID = "internal-user"

func internalPrincipalID() string {
	return internalPrincipalUserID
}

func defaultUser() map[string]any {
	return map[string]any{
		"id":          internalPrincipalID(),
		"email":       "internal@localhost",
		"displayName": "Internal User",
		"roles":       []string{"user"},
	}
}

func defaultUserWithQuota(gameCount, fileCount, storageUsedBytes, deviceCount int) map[string]any {
	u := defaultUser()
	u["storageUsedBytes"] = storageUsedBytes
	u["gameCount"] = gameCount
	u["fileCount"] = fileCount
	u["quota"] = map[string]any{
		"planName": "Internal",
		"planSlug": "internal",
		"storage": map[string]any{
			"current":   storageUsedBytes,
			"softLimit": 0,
			"hardLimit": 0,
			"status":    "ok",
		},
		"blobCount": map[string]any{
			"current":   fileCount,
			"softLimit": 0,
			"hardLimit": 0,
			"status":    "ok",
		},
		"savesPerGameSoftLimit": 0,
		"savesPerGameHardLimit": 0,
		"devices": map[string]any{
			"current":   deviceCount,
			"softLimit": 0,
			"hardLimit": 0,
			"status":    "ok",
		},
	}
	return u
}

func requestPrincipal(r *http.Request) map[string]any {
	tolerateNoAuthHeaders(r)
	return defaultUser()
}

func (a *app) currentUserWithQuota() map[string]any {
	snapshot := a.authSnapshot()
	return defaultUserWithQuota(snapshot.GameCount, snapshot.FileCount, snapshot.StorageUsedBytes, snapshot.DeviceCount)
}

func (a *app) currentRequestPrincipalWithQuota(r *http.Request) map[string]any {
	tolerateNoAuthHeaders(r)
	return a.currentUserWithQuota()
}
