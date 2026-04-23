package main

import "strings"

func buildTestN64Payload(ext, label string) []byte {
	size := 512
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "eep":
		size = 512
	case "fla":
		size = 131072
	case "sra", "mpk":
		size = 32768
	}

	payload := make([]byte, size)
	if label == "" {
		label = "n64-test"
	}
	seed := []byte(label)
	for i, value := range seed {
		payload[(i*131)%size] = value + byte((i%23)+1)
	}
	payload[size/2] = 0x5A
	payload[size-1] = 0xA5
	return payload
}
