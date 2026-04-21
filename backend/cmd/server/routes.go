package main

import (
	"net/http"
	"time"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

func newRouter(app *app) http.Handler {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Recoverer)
	r.Use(middleware.Logger)

	r.Get("/healthz", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]any{"ok": true, "time": time.Now().UTC()})
	})

	registerCompatRoutes(r, app, "")
	registerCompatRoutes(r, app, "/v1")

	return r
}

func registerCompatRoutes(r chi.Router, app *app, prefix string) {
	if prefix == "" {
		mountCompatRoutes(r, app)
		return
	}

	r.Route(prefix, func(r chi.Router) {
		mountCompatRoutes(r, app)
	})
}

func mountCompatRoutes(r chi.Router, app *app) {
	r.Get("/stripe/status", handleUnsupportedBillingAlias)

	r.Post("/auth/login", app.handleAuthLogin)
	r.Post("/auth/signup", app.handleAuthSignup)
	r.Post("/auth/logout", app.handleAuthLogout)
	r.Post("/auth/token", app.handleAuthToken)
	r.Post("/auth/token/app-password", app.handleAuthTokenAppPassword)
	r.Get("/auth/me", app.handleAuthMe)
	r.Post("/auth/resend-verification", app.handleAuthMessage)
	r.Post("/auth/verify-email", app.handleAuthMessage)
	r.Post("/auth/forgot-password", app.handleAuthMessage)
	r.Post("/auth/reset-password", app.handleAuthMessage)
	r.Post("/auth/delete-account", app.handleAuthMessage)
	r.Post("/auth/cancel-deletion", app.handleAuthMessage)
	r.Post("/auth/device", app.handleAuthDevice)
	r.Post("/auth/device/token", app.handleAuthDeviceToken)
	r.Post("/auth/device/verify", app.handleAuthDeviceVerify)
	r.Post("/auth/device/confirm", app.handleAuthDeviceConfirm)
	r.Post("/auth/2fa/verify", app.handleAuth2FAVerify)
	r.Post("/auth/2fa/setup/totp", app.handleAuth2FASetup)
	r.Post("/auth/2fa/verify-setup", app.handleAuth2FAVerifySetup)
	r.Get("/auth/2fa/status", app.handleAuth2FAStatus)
	r.Post("/auth/2fa/disable", app.handleAuth2FADisable)
	r.Post("/auth/2fa/backup-codes/regenerate", app.handleAuth2FABackupCodesRegenerate)
	r.Get("/auth/2fa/trusted-devices", app.handleAuth2FATrustedDevicesList)
	r.Delete("/auth/2fa/trusted-devices/{id}", app.handleAuth2FATrustedDevicesDelete)
	r.Get("/auth/app-passwords", app.handleAuthAppPasswordsList)
	r.Post("/auth/app-passwords", app.handleAuthAppPasswordsCreate)
	r.Delete("/auth/app-passwords/{id}", app.handleAuthAppPasswordsDelete)
	r.Get("/auth/app-passwords/auto-enroll", app.handleAuthAppPasswordsAutoStatus)
	r.Post("/auth/app-passwords/auto-enroll", app.handleAuthAppPasswordsAutoEnable)
	r.Get("/referral", app.handleReferral)
	r.Post("/dev/signup", app.handleDevSignup)

	r.Get("/save/latest", app.handleSaveLatest)
	r.Get("/save", app.handleSaveByGame)
	r.Post("/save/rollback", app.handleSaveRollback)
	r.Delete("/save", app.handleDeleteSave)

	r.Post("/saves", app.handleSaves)
	r.Get("/saves", app.handleListSaves)
	r.Delete("/saves", app.handleDeleteManySaves)
	r.Get("/saves/download", app.handleDownloadSave)
	r.Get("/saves/download_many", app.handleDownloadManySaves)
	r.Get("/saves/systems", app.handleSaveSystems)
	r.Delete("/game/saves", app.handleDeleteGameSaves)

	r.Get("/rom/lookup", app.handleRomLookup)
	r.Post("/rom/lookup", app.handleRomLookup)
	r.Get("/conflicts", app.handleConflictsList)
	r.Get("/conflicts/check", app.handleConflictsCheck)
	r.Post("/conflicts/check", app.handleConflictsCheck)
	r.Post("/conflicts/report", app.handleConflictsReport)
	r.Get("/conflicts/count", app.handleConflictsCount)
	r.Get("/conflicts/{id}", app.handleConflictsGet)
	r.Post("/conflicts/{id}/resolve", app.handleConflictsResolve)

	r.Post("/games/lookup", app.handleGamesLookup)
	r.Get("/games/library", app.handleGamesLibraryList)
	r.Post("/games/library", app.handleGamesLibraryCreate)
	r.Delete("/games/library/{id}", app.handleGamesLibraryDelete)
	r.Get("/catalog", app.handleCatalogList)
	r.Get("/catalog/{id}", app.handleCatalogGet)
	r.Get("/catalog/{id}/download", app.handleCatalogDownload)
	r.Get("/roadmap/items", app.handleRoadmapItemsList)
	r.Post("/roadmap/items/{id}/vote", app.handleRoadmapItemsVote)
	r.Post("/roadmap/suggestions", app.handleRoadmapSuggestionsCreate)
	r.Get("/roadmap/suggestions/mine", app.handleRoadmapSuggestionsMine)
	r.Get("/parser/wasm", app.handleParserWASM)

	r.Get("/devices", app.handleDevicesList)
	r.Get("/devices/{id}", app.handleDevicesGet)
	r.Patch("/devices/{id}", app.handleDevicesPatch)
	r.Delete("/devices/{id}", app.handleDevicesDelete)

	r.Get("/events", app.handleEvents)
}

func handleUnsupportedBillingAlias(w http.ResponseWriter, r *http.Request) {
	http.NotFound(w, r)
}
