package main

import "fmt"

const n64SmallEEPROMWindowSize = 0x200

func n64SmallEEPROMWindow(payload []byte, label string) ([]byte, error) {
	if len(payload) == n64SmallEEPROMWindowSize {
		return append([]byte(nil), payload...), nil
	}
	if len(payload) == n64CanonicalMediaByType["eeprom"].CanonicalSize {
		return append([]byte(nil), payload[:n64SmallEEPROMWindowSize]...), nil
	}
	return nil, fmt.Errorf("expected 512-byte or 2048-byte %s EEPROM, got %d", label, len(payload))
}

func n64PatchSmallEEPROMWindow(original []byte, window []byte, label string) ([]byte, error) {
	if len(window) != n64SmallEEPROMWindowSize {
		return nil, fmt.Errorf("expected patched 512-byte %s EEPROM window, got %d", label, len(window))
	}
	if len(original) == n64SmallEEPROMWindowSize {
		return append([]byte(nil), window...), nil
	}
	if len(original) == n64CanonicalMediaByType["eeprom"].CanonicalSize {
		out := append([]byte(nil), original...)
		copy(out[:n64SmallEEPROMWindowSize], window)
		return out, nil
	}
	return nil, fmt.Errorf("expected 512-byte or 2048-byte %s EEPROM, got %d", label, len(original))
}
