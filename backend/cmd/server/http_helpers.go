package main

import (
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strconv"
	"strings"
)

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func decodeJSONBody(r *http.Request, target any) error {
	if r == nil || r.Body == nil || target == nil {
		return nil
	}
	dec := json.NewDecoder(r.Body)
	if err := dec.Decode(target); err != nil && err != io.EOF {
		return err
	}
	return nil
}

func writeText(w http.ResponseWriter, status int, msg string) {
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.WriteHeader(status)
	_, _ = w.Write([]byte(msg))
}

func parseIntOrDefault(s string, d int) int {
	v, err := strconv.Atoi(s)
	if err != nil {
		return d
	}
	return v
}

func requireFile(form *multipart.Form, key string) error {
	if form == nil {
		return fmt.Errorf("missing form")
	}
	files := form.File[key]
	if len(files) == 0 {
		return fmt.Errorf("file is required")
	}
	return nil
}

func tolerateNoAuthHeaders(r *http.Request) {
	if r == nil {
		return
	}
	for _, key := range []string{"Authorization", "X-CSRF-Protection", "X-CSRF", "X-CSRF-Token"} {
		for _, value := range r.Header.Values(key) {
			_ = strings.TrimSpace(value)
		}
	}
}
