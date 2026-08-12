package main

import (
	"fmt"
	"mime"
	"net/http"
	"os"
	"strconv"
	"strings"
)

const defaultMaxMultipartBodyBytes int64 = 128 << 20

// maxMultipartBodyBytes returns the maximum accepted multipart request size,
// including MIME boundaries and form fields. Operators can raise or lower the
// default with MAX_MULTIPART_BODY_BYTES.
func maxMultipartBodyBytes() int64 {
	raw := strings.TrimSpace(os.Getenv("MAX_MULTIPART_BODY_BYTES"))
	if raw == "" {
		return defaultMaxMultipartBodyBytes
	}

	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil || value <= 0 {
		return defaultMaxMultipartBodyBytes
	}
	return value
}

func isMultipartFormRequest(r *http.Request) bool {
	if r == nil {
		return false
	}

	contentType := strings.TrimSpace(r.Header.Get("Content-Type"))
	if contentType == "" {
		return false
	}

	mediaType, _, err := mime.ParseMediaType(contentType)
	if err != nil {
		// Invalid multipart headers still need a body limit. The downstream
		// handler remains responsible for returning the syntax error.
		return strings.HasPrefix(strings.ToLower(contentType), "multipart/form-data")
	}
	return strings.EqualFold(mediaType, "multipart/form-data")
}

// limitMultipartRequestBody places a hard cap on multipart requests before a
// handler calls ParseMultipartForm or reads a file into memory. ParseMultipartForm's
// maxMemory argument is not a total upload limit: larger parts are written to
// temporary files. MaxBytesReader also protects requests without Content-Length.
func limitMultipartRequestBody(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r == nil || r.Body == nil || !isMultipartFormRequest(r) {
			next.ServeHTTP(w, r)
			return
		}

		limit := maxMultipartBodyBytes()
		if r.ContentLength > limit {
			writeJSON(w, http.StatusRequestEntityTooLarge, apiError{
				Error:      "Payload Too Large",
				Message:    fmt.Sprintf("multipart request exceeds the %d byte limit", limit),
				StatusCode: http.StatusRequestEntityTooLarge,
			})
			return
		}

		r.Body = http.MaxBytesReader(w, r.Body, limit)
		next.ServeHTTP(w, r)
	})
}
