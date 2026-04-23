package main

import (
	"bytes"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	n64ProfileMister      = "n64/mister"
	n64ProfileRetroArch   = "n64/retroarch"
	n64ProfileProject64   = "n64/project64"
	n64ProfileMupenFamily = "n64/mupen-family"
	n64ProfileEverDrive   = "n64/everdrive"

	n64RetroArchSRMSize           = 0x48800
	n64RetroArchEEPROMOffset      = 0x00000
	n64RetroArchEEPROMSize        = 0x00800
	n64RetroArchControllerOffset  = 0x00800
	n64RetroArchControllerSize    = 0x20000
	n64RetroArchControllerPakSize = 0x08000
	n64RetroArchSRAMOffset        = 0x20800
	n64RetroArchSRAMSize          = 0x08000
	n64RetroArchFlashRAMOffset    = 0x28800
	n64RetroArchFlashRAMSize      = 0x20000
)

type n64MediaInfo struct {
	MediaType     string
	Extension     string
	CanonicalSize int
}

var n64CanonicalMediaByType = map[string]n64MediaInfo{
	"eeprom":         {MediaType: "eeprom", Extension: "eep", CanonicalSize: 2048},
	"sram":           {MediaType: "sram", Extension: "sra", CanonicalSize: 32768},
	"flashram":       {MediaType: "flashram", Extension: "fla", CanonicalSize: 131072},
	"controller-pak": {MediaType: "controller-pak", Extension: "mpk", CanonicalSize: 32768},
}

func canonicalN64Profile(value string) string {
	switch strings.ToLower(strings.TrimSpace(value)) {
	case "n64/mister", "mister":
		return n64ProfileMister
	case "n64/retroarch", "retroarch", "retro-arch":
		return n64ProfileRetroArch
	case "n64/project64", "project64", "pj64":
		return n64ProfileProject64
	case "n64/mupen-family", "mupen-family", "mupen", "mupen64plus", "rmg", "rosalies-mupen-gui", "rosalie's mupen gui":
		return n64ProfileMupenFamily
	case "n64/everdrive", "everdrive", "ever-drive":
		return n64ProfileEverDrive
	default:
		return ""
	}
}

func requiresN64ProfileForHelper(systemSlug string, helper bool) bool {
	return helper && canonicalSegment(systemSlug, "") == "n64"
}

func n64SummaryMediaInfo(summary saveSummary) (n64MediaInfo, bool) {
	if info, ok := n64CanonicalMediaByType[strings.TrimSpace(summary.MediaType)]; ok {
		return info, true
	}
	if summary.Inspection != nil {
		if mediaType, _ := summary.Inspection.SemanticFields["mediaType"].(string); mediaType != "" {
			if info, ok := n64CanonicalMediaByType[strings.TrimSpace(mediaType)]; ok {
				return info, true
			}
		}
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(summary.Filename))), ".")
	return n64MediaInfoFromExtensionAndSize(ext, summary.FileSize)
}

func decorateN64ProjectionFields(input *saveCreateInput) {
	if input == nil {
		return
	}
	systemCandidate := input.SystemSlug
	if systemCandidate == "" && input.Game.System != nil {
		systemCandidate = input.Game.System.Slug
	}
	systemSlug := canonicalSegment(systemCandidate, "")
	if systemSlug != "n64" {
		return
	}
	if strings.TrimSpace(input.MediaType) == "" && input.Inspection != nil {
		if mediaType, _ := input.Inspection.SemanticFields["mediaType"].(string); mediaType != "" {
			input.MediaType = strings.TrimSpace(mediaType)
		}
	}
	if input.ProjectionCapable == nil {
		capable := true
		input.ProjectionCapable = &capable
	}
	if strings.TrimSpace(input.SourceArtifactProfile) == "" {
		input.SourceArtifactProfile = strings.TrimSpace(input.RuntimeProfile)
	}
}

func n64MediaInfoFromExtensionAndSize(ext string, size int) (n64MediaInfo, bool) {
	switch strings.ToLower(strings.TrimSpace(ext)) {
	case "eep":
		if size == 512 || size == 2048 {
			return n64CanonicalMediaByType["eeprom"], true
		}
	case "sra":
		if size == 32768 {
			return n64CanonicalMediaByType["sram"], true
		}
	case "fla":
		if size == 131072 {
			return n64CanonicalMediaByType["flashram"], true
		}
	case "mpk":
		if size == 32768 {
			return n64CanonicalMediaByType["controller-pak"], true
		}
	}
	return n64MediaInfo{}, false
}

func normalizeN64ProjectionUpload(input saveCreateInput, requestedProfile string) (saveCreateInput, error) {
	profile := canonicalN64Profile(requestedProfile)
	if profile == "" {
		return input, fmt.Errorf("n64Profile is required for N64 helper uploads")
	}
	stem := strings.TrimSpace(strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename)))
	if stem == "" {
		stem = "n64-save"
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(input.Filename))), ".")
	canonicalPayload, info, err := normalizeN64UploadPayload(profile, ext, input.Payload)
	if err != nil {
		return input, err
	}
	capable := true
	input.Payload = canonicalPayload
	input.Filename = stem + "." + info.Extension
	input.Format = inferSaveFormat(input.Filename)
	input.MediaType = info.MediaType
	input.ProjectionCapable = &capable
	input.SourceArtifactProfile = profile
	input.RuntimeProfile = profile
	return input, nil
}

func normalizeN64UploadPayload(profile, ext string, payload []byte) ([]byte, n64MediaInfo, error) {
	profile = canonicalN64Profile(profile)
	ext = strings.ToLower(strings.TrimSpace(ext))
	if profile == "" {
		return nil, n64MediaInfo{}, fmt.Errorf("unsupported N64 runtime profile")
	}
	if profile == n64ProfileRetroArch && ext == "srm" {
		return splitRetroArchN64SRM(payload)
	}
	if profile == n64ProfileMupenFamily && ext == "mpk" && len(payload) == n64RetroArchControllerSize {
		canonical, err := splitMergedMupenControllerPak(payload)
		if err != nil {
			return nil, n64MediaInfo{}, err
		}
		return canonical, n64CanonicalMediaByType["controller-pak"], nil
	}
	if profile == n64ProfileEverDrive && ext == "srm" && len(payload) == 32768 {
		return append([]byte(nil), payload...), n64CanonicalMediaByType["sram"], nil
	}

	info, ok := n64MediaInfoFromExtensionAndSize(ext, len(payload))
	if !ok {
		return nil, n64MediaInfo{}, fmt.Errorf("unsupported N64 upload form for %s: .%s (%d bytes)", profile, ext, len(payload))
	}

	canonical := append([]byte(nil), payload...)
	switch info.MediaType {
	case "eeprom":
		canonical = normalizeN64EEPROM(canonical)
	case "sram", "flashram":
		if profile == n64ProfileProject64 || profile == n64ProfileMupenFamily || profile == n64ProfileRetroArch {
			canonical = n64Swap32Words(canonical)
		}
	}
	return canonical, info, nil
}

func projectN64Payload(summary saveSummary, payload []byte, requestedProfile string) (string, string, []byte, error) {
	profile := canonicalN64Profile(requestedProfile)
	if profile == "" {
		return "", "", nil, fmt.Errorf("n64Profile is required for N64 downloads")
	}
	info, ok := n64SummaryMediaInfo(summary)
	if !ok {
		return "", "", nil, fmt.Errorf("N64 save media type is unknown")
	}
	if len(payload) != info.CanonicalSize {
		return "", "", nil, fmt.Errorf("N64 canonical payload size %d does not match expected %d for %s", len(payload), info.CanonicalSize, info.MediaType)
	}
	stem := strings.TrimSpace(strings.TrimSuffix(summary.Filename, filepath.Ext(summary.Filename)))
	if stem == "" {
		stem = canonicalSegment(summary.DisplayTitle, "n64-save")
	}
	if stem == "" {
		stem = "n64-save"
	}
	filenameExt := info.Extension
	projected := append([]byte(nil), payload...)

	switch profile {
	case n64ProfileMister:
		// Native canonical order.
	case n64ProfileEverDrive:
		if info.MediaType == "sram" {
			filenameExt = "srm"
		}
	case n64ProfileProject64:
		if info.MediaType == "sram" || info.MediaType == "flashram" {
			projected = n64Swap32Words(projected)
		}
	case n64ProfileMupenFamily:
		if info.MediaType == "sram" || info.MediaType == "flashram" {
			projected = n64Swap32Words(projected)
		}
		if info.MediaType == "controller-pak" {
			projected = buildMergedMupenControllerPak(projected)
		}
	case n64ProfileRetroArch:
		projected = buildRetroArchN64SRM(info, projected)
		filenameExt = "srm"
	default:
		return "", "", nil, fmt.Errorf("unsupported N64 runtime profile %q", requestedProfile)
	}

	return stem + "." + filenameExt, "application/octet-stream", projected, nil
}

func normalizeN64EEPROM(payload []byte) []byte {
	if len(payload) >= 2048 {
		return append([]byte(nil), payload[:2048]...)
	}
	out := bytes.Repeat([]byte{0xFF}, 2048)
	copy(out, payload)
	return out
}

func splitRetroArchN64SRM(payload []byte) ([]byte, n64MediaInfo, error) {
	if len(payload) != n64RetroArchSRMSize {
		return nil, n64MediaInfo{}, fmt.Errorf("retroarch N64 SRM must be %d bytes", n64RetroArchSRMSize)
	}

	type candidate struct {
		info    n64MediaInfo
		payload []byte
	}
	candidates := make([]candidate, 0, 4)

	eeprom := payload[n64RetroArchEEPROMOffset : n64RetroArchEEPROMOffset+n64RetroArchEEPROMSize]
	if !allBytesEqual(eeprom, 0xFF) {
		candidates = append(candidates, candidate{info: n64CanonicalMediaByType["eeprom"], payload: normalizeN64EEPROM(eeprom)})
	}

	nonEmptyPacks := make([][]byte, 0, 4)
	for i := 0; i < 4; i++ {
		offset := n64RetroArchControllerOffset + i*n64RetroArchControllerPakSize
		pack := payload[offset : offset+n64RetroArchControllerPakSize]
		if !n64ControllerPakIsEmpty(pack) {
			nonEmptyPacks = append(nonEmptyPacks, append([]byte(nil), pack...))
		}
	}
	if len(nonEmptyPacks) > 1 {
		return nil, n64MediaInfo{}, fmt.Errorf("retroarch N64 SRM contains multiple populated controller paks")
	}
	if len(nonEmptyPacks) == 1 {
		candidates = append(candidates, candidate{info: n64CanonicalMediaByType["controller-pak"], payload: nonEmptyPacks[0]})
	}

	sram := payload[n64RetroArchSRAMOffset : n64RetroArchSRAMOffset+n64RetroArchSRAMSize]
	if !allBytesEqual(sram, 0xFF) {
		candidates = append(candidates, candidate{info: n64CanonicalMediaByType["sram"], payload: n64Swap32Words(sram)})
	}

	flashram := payload[n64RetroArchFlashRAMOffset : n64RetroArchFlashRAMOffset+n64RetroArchFlashRAMSize]
	if !allBytesEqual(flashram, 0xFF) {
		candidates = append(candidates, candidate{info: n64CanonicalMediaByType["flashram"], payload: n64Swap32Words(flashram)})
	}

	if len(candidates) == 0 {
		return nil, n64MediaInfo{}, fmt.Errorf("retroarch N64 SRM does not contain a populated save payload")
	}
	if len(candidates) > 1 {
		labels := make([]string, 0, len(candidates))
		for _, candidate := range candidates {
			labels = append(labels, candidate.info.MediaType)
		}
		return nil, n64MediaInfo{}, fmt.Errorf("retroarch N64 SRM contains multiple populated save classes: %s", strings.Join(labels, ", "))
	}
	return candidates[0].payload, candidates[0].info, nil
}

func splitMergedMupenControllerPak(payload []byte) ([]byte, error) {
	if len(payload) != n64RetroArchControllerSize {
		return nil, fmt.Errorf("mupen controller pak must be %d bytes", n64RetroArchControllerSize)
	}
	nonEmpty := make([][]byte, 0, 4)
	for i := 0; i < 4; i++ {
		offset := i * n64RetroArchControllerPakSize
		pack := payload[offset : offset+n64RetroArchControllerPakSize]
		if !n64ControllerPakIsEmpty(pack) {
			nonEmpty = append(nonEmpty, append([]byte(nil), pack...))
		}
	}
	if len(nonEmpty) == 0 {
		return nil, fmt.Errorf("mupen controller pak does not contain a populated controller save")
	}
	if len(nonEmpty) > 1 {
		return nil, fmt.Errorf("mupen controller pak contains multiple populated controller packs")
	}
	return nonEmpty[0], nil
}

func buildRetroArchN64SRM(info n64MediaInfo, canonical []byte) []byte {
	out := bytes.Repeat([]byte{0xFF}, n64RetroArchSRMSize)
	for i := 0; i < 4; i++ {
		offset := n64RetroArchControllerOffset + i*n64RetroArchControllerPakSize
		n64InitControllerPak(out[offset : offset+n64RetroArchControllerPakSize])
	}

	switch info.MediaType {
	case "eeprom":
		copy(out[n64RetroArchEEPROMOffset:n64RetroArchEEPROMOffset+n64RetroArchEEPROMSize], normalizeN64EEPROM(canonical))
	case "controller-pak":
		copy(out[n64RetroArchControllerOffset:n64RetroArchControllerOffset+n64RetroArchControllerPakSize], canonical)
	case "sram":
		copy(out[n64RetroArchSRAMOffset:n64RetroArchSRAMOffset+n64RetroArchSRAMSize], n64Swap32Words(canonical))
	case "flashram":
		copy(out[n64RetroArchFlashRAMOffset:n64RetroArchFlashRAMOffset+n64RetroArchFlashRAMSize], n64Swap32Words(canonical))
	}
	return out
}

func buildMergedMupenControllerPak(canonical []byte) []byte {
	out := make([]byte, n64RetroArchControllerSize)
	for i := 0; i < 4; i++ {
		offset := i * n64RetroArchControllerPakSize
		n64InitControllerPak(out[offset : offset+n64RetroArchControllerPakSize])
	}
	copy(out[:n64RetroArchControllerPakSize], canonical)
	return out
}

func n64ControllerPakIsEmpty(buf []byte) bool {
	if len(buf) < 512 {
		return true
	}
	table := buf[256:512]
	for i := 10; i+1 < len(table); i += 2 {
		if table[i] != 0x00 || table[i+1] != 0x03 {
			return false
		}
	}
	return true
}

func n64InitControllerPak(buf []byte) {
	const freeSpaceHi = 0x00
	const freeSpaceLo = 0x03
	serial := []byte{
		0xff, 0xff, 0xff, 0xff, 0x05, 0x1a, 0x5f, 0x13,
		0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00, 0x00,
		0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff, 0xff,
	}
	for i := range buf {
		buf[i] = 0
	}
	if len(buf) < n64RetroArchControllerPakSize {
		return
	}
	buf[0] = 0x81
	for i := 1; i < 32; i++ {
		buf[i] = byte(i)
	}
	copy(buf[32:56], serial)
	buf[56] = 0xff
	buf[57] = 0xff
	buf[58] = 0x01
	buf[59] = 0xff
	checksum1 := n64ControllerChecksum1(buf[32:64])
	copy(buf[60:62], checksum1[:])
	checksum2 := n64ControllerChecksum2(checksum1)
	copy(buf[62:64], checksum2[:])
	copy(buf[96:128], buf[32:64])
	copy(buf[128:160], buf[32:64])
	copy(buf[192:224], buf[32:64])

	table := buf[256:512]
	table[0] = 0x00
	table[1] = 0x71
	for i := 10; i < len(table); i += 2 {
		table[i] = freeSpaceHi
		if i+1 < len(table) {
			table[i+1] = freeSpaceLo
		}
	}
	copy(buf[512:768], table)
}

func n64ControllerChecksum1(buf []byte) [2]byte {
	var sum uint16
	for i := 0; i+1 < 24; i += 2 {
		sum += uint16(buf[i])<<8 | uint16(buf[i+1])
	}
	sum += uint16(buf[24])<<8 | uint16(buf[25])
	sum += uint16(buf[26])<<8 | uint16(buf[27])
	return [2]byte{byte(sum >> 8), byte(sum)}
}

func n64ControllerChecksum2(buf [2]byte) [2]byte {
	sum := uint16(0xfff2) - (uint16(buf[0])<<8 | uint16(buf[1]))
	return [2]byte{byte(sum >> 8), byte(sum)}
}
