package main

import (
	"crypto/rand"
	"encoding/hex"
)

func randomHex(bytes int) string {
	buf := make([]byte, bytes)
	if _, err := rand.Read(buf); err != nil {
		return "seed"
	}
	return hex.EncodeToString(buf)
}
