package main

import (
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
)

func randomHex(byteCount int) (string, error) {
	return randomHexFrom(rand.Reader, byteCount)
}

func randomHexFrom(reader io.Reader, byteCount int) (string, error) {
	if byteCount <= 0 {
		return "", fmt.Errorf("random byte count must be positive")
	}
	if reader == nil {
		return "", fmt.Errorf("random reader is required")
	}

	buf := make([]byte, byteCount)
	if _, err := io.ReadFull(reader, buf); err != nil {
		return "", fmt.Errorf("read cryptographic randomness: %w", err)
	}
	return hex.EncodeToString(buf), nil
}
