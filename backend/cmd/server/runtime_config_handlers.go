package main

import (
	"net/http"
	"os"
	"strings"
)

func (a *app) handleRuntimeConfig(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)
	mode := authMode()
	writeJSON(w, http.StatusOK, runtimeConfigResponse{
		Success: true,
		Runtime: runtimeConfig{
			AppName:     "RetroSaveManager",
			AuthMode:    mode,
			AuthEnabled: mode != "disabled",
			BaseURL:     baseURLForRequest(r),
			Version:     runtimeConfigValue("RSM_VERSION", "dev"),
			Commit:      runtimeConfigValue("RSM_COMMIT", ""),
			Features: runtimeConfigFeatures{
				SelfHosted:       true,
				PublicSignup:     false,
				HelperPairing:    true,
				SaveValidation:   true,
				RuntimeModules:   true,
				CloudMultiTenant: false,
			},
			Warnings: runtimeConfigWarnings(mode),
		},
	})
}

func runtimeConfigValue(key, fallback string) string {
	value := strings.TrimSpace(os.Getenv(key))
	if value == "" {
		return fallback
	}
	return value
}

func runtimeConfigWarnings(mode string) []string {
	if mode == "disabled" {
		return []string{"Authentication is disabled. Keep this instance on a trusted LAN or protect it behind your own reverse proxy."}
	}
	return []string{}
}
