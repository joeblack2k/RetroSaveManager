package main

import (
	"crypto/md5"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"net/http"
	"strings"
)

func (a *app) handleRomLookup(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	stem := romLookupStemFromRequest(r)
	if stem == "" {
		stem = "Unknown"
	}

	gameID := deterministicGameID(stem)
	if strings.EqualFold(stem, "Wario Land II") {
		gameID = 281
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"success": true,
		"count":   1,
		"rom": map[string]any{
			"id":       gameID,
			"sha1":     deterministicSHA1Hex("rom:" + stem),
			"md5":      deterministicMD5Hex("rom:" + stem),
			"fileName": stem + ".srm",
			"game": map[string]any{
				"id":          gameID,
				"name":        stem,
				"boxart":      nil,
				"boxartThumb": nil,
				"hasParser":   false,
				"system":      nil,
			},
		},
	})
}

func romLookupStemFromRequest(r *http.Request) string {
	if stem := strings.TrimSpace(r.URL.Query().Get("filenameStem")); stem != "" {
		return stem
	}
	if r.Method != http.MethodPost {
		return ""
	}

	if strings.Contains(r.Header.Get("Content-Type"), "application/json") {
		var req struct {
			FilenameStem string `json:"filenameStem"`
		}
		if err := json.NewDecoder(r.Body).Decode(&req); err == nil {
			return strings.TrimSpace(req.FilenameStem)
		}
		return ""
	}

	if err := r.ParseForm(); err == nil {
		return strings.TrimSpace(r.FormValue("filenameStem"))
	}
	return ""
}

func deterministicSHA1Hex(value string) string {
	sum := sha1.Sum([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(sum[:])
}

func deterministicMD5Hex(value string) string {
	sum := md5.Sum([]byte(strings.ToLower(strings.TrimSpace(value))))
	return hex.EncodeToString(sum[:])
}

func (a *app) handleGamesLookup(w http.ResponseWriter, r *http.Request) {
	_ = requestPrincipal(r)

	var req gamesLookupRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.Items == nil {
		writeText(w, http.StatusUnprocessableEntity, "Failed to deserialize the JSON body into the target type: missing field `items`")
		return
	}

	results := make([]map[string]any, 0, len(req.Items))
	for _, item := range req.Items {
		name := "Unknown Game"
		var v map[string]any
		if err := json.Unmarshal(item.Value, &v); err == nil {
			if s, ok := v["name"].(string); ok && s != "" {
				name = s
			}
		}

		results = append(results, map[string]any{
			"query": item,
			"game": map[string]any{
				"id":          281,
				"name":        name,
				"boxart":      nil,
				"boxartThumb": nil,
				"hasParser":   false,
				"system":      nil,
			},
		})
	}

	writeJSON(w, http.StatusOK, map[string]any{"results": results})
}
