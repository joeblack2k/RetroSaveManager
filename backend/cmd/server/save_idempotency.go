package main

import (
	"bytes"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"sync"
	"time"
)

const (
	saveIdempotencyStateFileName = "save_idempotency.json"
	saveIdempotencyRetention     = 7 * 24 * time.Hour
	saveIdempotencyMaxEntries    = 4096
	idempotencyKeyMaxLength      = 200
)

type saveIdempotencyRecord struct {
	KeyHash            string            `json:"keyHash"`
	RequestFingerprint string            `json:"requestFingerprint"`
	StatusCode         int               `json:"statusCode"`
	ResponseHeaders    map[string]string `json:"responseHeaders,omitempty"`
	ResponseBody       json.RawMessage   `json:"responseBody"`
	CreatedAt          time.Time         `json:"createdAt"`
}

type saveIdempotencyStateFile struct {
	Records   []saveIdempotencyRecord `json:"records"`
	UpdatedAt time.Time               `json:"updatedAt"`
}

var saveIdempotencyMu sync.Mutex

type bufferedHTTPResponse struct {
	header http.Header
	body   bytes.Buffer
	status int
}

func newBufferedHTTPResponse() *bufferedHTTPResponse {
	return &bufferedHTTPResponse{header: make(http.Header), status: http.StatusOK}
}

func (w *bufferedHTTPResponse) Header() http.Header { return w.header }
func (w *bufferedHTTPResponse) WriteHeader(status int) {
	if w.status != http.StatusOK || w.body.Len() > 0 {
		return
	}
	w.status = status
}
func (w *bufferedHTTPResponse) Write(p []byte) (int, error) { return w.body.Write(p) }

func saveIdempotencyStatePath() string {
	return filepath.Join(stateRootDirFromEnv(), saveIdempotencyStateFileName)
}

func normalizeIdempotencyKey(raw string) (string, error) {
	key := strings.TrimSpace(raw)
	if key == "" {
		return "", nil
	}
	if len(key) > idempotencyKeyMaxLength {
		return "", fmt.Errorf("Idempotency-Key exceeds %d characters", idempotencyKeyMaxLength)
	}
	for _, r := range key {
		if r < 0x21 || r > 0x7e {
			return "", fmt.Errorf("Idempotency-Key contains unsupported characters")
		}
	}
	return key, nil
}

func idempotencyKeyHash(key string) string {
	sum := sha256.Sum256([]byte(key))
	return hex.EncodeToString(sum[:])
}

func saveIdempotencyCanonicalPath(r *http.Request) string {
	if r == nil {
		return ""
	}
	return stripRoutePrefix(r.URL.Path)
}

func isIdempotentSaveUploadRequest(r *http.Request) bool {
	return r != nil && r.Method == http.MethodPost && saveIdempotencyCanonicalPath(r) == "/saves"
}

func saveUploadRequestFingerprint(r *http.Request) (string, error) {
	if r == nil {
		return "", fmt.Errorf("request is required")
	}
	mediaType := r.Header.Get("Content-Type")
	if !strings.Contains(strings.ToLower(mediaType), "multipart/form-data") {
		return "", fmt.Errorf("Idempotency-Key is only supported for multipart save uploads")
	}
	if err := r.ParseMultipartForm(64 << 20); err != nil {
		return "", fmt.Errorf("parse multipart request for idempotency: %w", err)
	}
	if r.MultipartForm == nil {
		return "", fmt.Errorf("multipart form is unavailable")
	}

	h := sha256.New()
	_, _ = io.WriteString(h, r.Method+"\n"+saveIdempotencyCanonicalPath(r)+"\n")

	valueKeys := make([]string, 0, len(r.MultipartForm.Value))
	for key := range r.MultipartForm.Value {
		valueKeys = append(valueKeys, key)
	}
	sort.Strings(valueKeys)
	for _, key := range valueKeys {
		values := append([]string(nil), r.MultipartForm.Value[key]...)
		sort.Strings(values)
		for _, value := range values {
			_, _ = io.WriteString(h, "v\x00"+key+"\x00"+value+"\n")
		}
	}

	fileKeys := make([]string, 0, len(r.MultipartForm.File))
	for key := range r.MultipartForm.File {
		fileKeys = append(fileKeys, key)
	}
	sort.Strings(fileKeys)
	for _, key := range fileKeys {
		files := r.MultipartForm.File[key]
		for _, header := range files {
			file, err := header.Open()
			if err != nil {
				return "", fmt.Errorf("open multipart file for idempotency: %w", err)
			}
			fileHash := sha256.New()
			_, copyErr := io.Copy(fileHash, file)
			closeErr := file.Close()
			if copyErr != nil {
				return "", fmt.Errorf("hash multipart file for idempotency: %w", copyErr)
			}
			if closeErr != nil {
				return "", fmt.Errorf("close multipart file for idempotency: %w", closeErr)
			}
			_, _ = io.WriteString(h, "f\x00"+key+"\x00"+safeFilename(header.Filename)+"\x00"+hex.EncodeToString(fileHash.Sum(nil))+"\n")
		}
	}
	return hex.EncodeToString(h.Sum(nil)), nil
}

func loadSaveIdempotencyStateLocked(now time.Time) (saveIdempotencyStateFile, error) {
	state := saveIdempotencyStateFile{Records: []saveIdempotencyRecord{}}
	data, err := os.ReadFile(saveIdempotencyStatePath())
	if err != nil {
		if os.IsNotExist(err) {
			return state, nil
		}
		return state, fmt.Errorf("read save idempotency state: %w", err)
	}
	if len(data) == 0 {
		return state, nil
	}
	if err := json.Unmarshal(data, &state); err != nil {
		return saveIdempotencyStateFile{}, fmt.Errorf("decode save idempotency state: %w", err)
	}
	cutoff := now.UTC().Add(-saveIdempotencyRetention)
	kept := state.Records[:0]
	for _, record := range state.Records {
		if record.CreatedAt.IsZero() || record.CreatedAt.Before(cutoff) || record.KeyHash == "" || record.RequestFingerprint == "" {
			continue
		}
		kept = append(kept, record)
	}
	state.Records = kept
	sort.Slice(state.Records, func(i, j int) bool { return state.Records[i].CreatedAt.After(state.Records[j].CreatedAt) })
	if len(state.Records) > saveIdempotencyMaxEntries {
		state.Records = state.Records[:saveIdempotencyMaxEntries]
	}
	return state, nil
}

func persistSaveIdempotencyStateLocked(state saveIdempotencyStateFile, now time.Time) error {
	state.UpdatedAt = now.UTC()
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode save idempotency state: %w", err)
	}
	if err := writeFileAtomic(saveIdempotencyStatePath(), data, 0o600); err != nil {
		return fmt.Errorf("write save idempotency state: %w", err)
	}
	return nil
}

func findSaveIdempotencyRecord(state saveIdempotencyStateFile, keyHash string) (saveIdempotencyRecord, bool) {
	for _, record := range state.Records {
		if record.KeyHash == keyHash {
			return record, true
		}
	}
	return saveIdempotencyRecord{}, false
}

func replaySaveIdempotencyResponse(w http.ResponseWriter, record saveIdempotencyRecord) {
	for key, value := range record.ResponseHeaders {
		w.Header().Set(key, value)
	}
	w.Header().Set("X-RSM-Idempotent-Replay", "true")
	w.WriteHeader(record.StatusCode)
	_, _ = w.Write(record.ResponseBody)
}

func flushBufferedHTTPResponse(w http.ResponseWriter, buffered *bufferedHTTPResponse) {
	for key, values := range buffered.Header() {
		for _, value := range values {
			w.Header().Add(key, value)
		}
	}
	w.WriteHeader(buffered.status)
	_, _ = w.Write(buffered.body.Bytes())
}

func responseHeadersForIdempotency(header http.Header) map[string]string {
	out := map[string]string{}
	for _, key := range []string{"Content-Type", "ETag", conditionalWritesHeader} {
		if value := strings.TrimSpace(header.Get(key)); value != "" {
			out[key] = value
		}
	}
	return out
}

func (a *app) enforceSaveUploadIdempotency(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if !isIdempotentSaveUploadRequest(r) {
			next.ServeHTTP(w, r)
			return
		}
		key, err := normalizeIdempotencyKey(r.Header.Get("Idempotency-Key"))
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
			return
		}
		if key == "" {
			next.ServeHTTP(w, r)
			return
		}

		fingerprint, err := saveUploadRequestFingerprint(r)
		if err != nil {
			writeJSON(w, http.StatusBadRequest, apiError{Error: "Bad Request", Message: err.Error(), StatusCode: http.StatusBadRequest})
			return
		}
		keyHash := idempotencyKeyHash(key)
		now := time.Now().UTC()

		// Serializing keyed upload execution behind this dedicated mutex avoids
		// the race where two simultaneous requests with the same key both mutate
		// storage before either result reaches the durable ledger.
		saveIdempotencyMu.Lock()
		defer saveIdempotencyMu.Unlock()

		state, err := loadSaveIdempotencyStateLocked(now)
		if err != nil {
			writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: "unable to read idempotency state", StatusCode: http.StatusInternalServerError})
			return
		}
		if existing, found := findSaveIdempotencyRecord(state, keyHash); found {
			if existing.RequestFingerprint != fingerprint {
				writeJSON(w, http.StatusConflict, apiError{Error: "Conflict", Message: "Idempotency-Key was already used for a different save upload", StatusCode: http.StatusConflict})
				return
			}
			replaySaveIdempotencyResponse(w, existing)
			return
		}

		buffered := newBufferedHTTPResponse()
		next.ServeHTTP(buffered, r)
		if buffered.status >= 200 && buffered.status < 300 && json.Valid(buffered.body.Bytes()) {
			record := saveIdempotencyRecord{
				KeyHash:            keyHash,
				RequestFingerprint: fingerprint,
				StatusCode:         buffered.status,
				ResponseHeaders:    responseHeadersForIdempotency(buffered.Header()),
				ResponseBody:       append(json.RawMessage(nil), buffered.body.Bytes()...),
				CreatedAt:          now,
			}
			state.Records = append([]saveIdempotencyRecord{record}, state.Records...)
			if len(state.Records) > saveIdempotencyMaxEntries {
				state.Records = state.Records[:saveIdempotencyMaxEntries]
			}
			if err := persistSaveIdempotencyStateLocked(state, now); err != nil {
				writeJSON(w, http.StatusInternalServerError, apiError{Error: "Internal Server Error", Message: "save upload completed but idempotency result could not be persisted; retry with the same key", StatusCode: http.StatusInternalServerError})
				return
			}
		}
		flushBufferedHTTPResponse(w, buffered)
	})
}

// multipart.FileHeader.Open can create temporary files through ParseMultipartForm.
// The request owner removes them after the handler chain has consumed the form.
func cleanupMultipartForm(r *http.Request) {
	if r != nil && r.MultipartForm != nil {
		_ = r.MultipartForm.RemoveAll()
	}
}

var _ = multipart.ErrMessageTooLarge
