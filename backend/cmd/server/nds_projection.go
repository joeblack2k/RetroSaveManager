package main

import (
	"bytes"
	"encoding/binary"
	"fmt"
	"path/filepath"
	"strings"
)

const (
	desmumeBackupFooterText = "|<--Snip above here to create a raw sav by excluding this DeSmuME savedata footer:"
	desmumeSaveCookie       = "|-DESMUME SAVE-|"
	desmumeFooterSize       = len(desmumeBackupFooterText) + 40

	noGBASaveHeaderID   = "NocashGbaBackupMediaSavDataFile"
	noGBASaveHeaderMark = 0x1A
	noGBASaveSRAMID     = "SRAM"
)

var ndsRawSaveSizeOrder = []int{
	512,
	8 * 1024,
	32 * 1024,
	64 * 1024,
	256 * 1024,
	512 * 1024,
	1024 * 1024,
	2048 * 1024,
	4096 * 1024,
	8192 * 1024,
	16384 * 1024,
	32768 * 1024,
	65536 * 1024,
}

var ndsRawSaveSizes = func() map[int]struct{} {
	out := make(map[int]struct{}, len(ndsRawSaveSizeOrder))
	for _, size := range ndsRawSaveSizeOrder {
		out[size] = struct{}{}
	}
	return out
}()

type ndsSaveType struct {
	Size     int
	Type     uint32
	AddrSize uint32
}

var ndsDesmumeSaveTypes = []ndsSaveType{
	{Size: 512, Type: 1, AddrSize: 1},
	{Size: 8 * 1024, Type: 2, AddrSize: 2},
	{Size: 64 * 1024, Type: 3, AddrSize: 2},
	{Size: 32 * 1024, Type: 4, AddrSize: 2},
	{Size: 256 * 1024, Type: 5, AddrSize: 3},
	{Size: 512 * 1024, Type: 6, AddrSize: 3},
	{Size: 1024 * 1024, Type: 7, AddrSize: 3},
	{Size: 2048 * 1024, Type: 8, AddrSize: 3},
	{Size: 4096 * 1024, Type: 9, AddrSize: 3},
	{Size: 8192 * 1024, Type: 10, AddrSize: 3},
	{Size: 16384 * 1024, Type: 11, AddrSize: 3},
	{Size: 32768 * 1024, Type: 12, AddrSize: 3},
	{Size: 65536 * 1024, Type: 13, AddrSize: 3},
}

var ndsDesmumeSaveTypesBySize = func() map[int]ndsSaveType {
	out := make(map[int]ndsSaveType, len(ndsDesmumeSaveTypes))
	for _, saveType := range ndsDesmumeSaveTypes {
		out[saveType.Size] = saveType
	}
	return out
}()

func normalizeNDSProjectionUpload(input saveCreateInput, requestedProfile string) (saveCreateInput, error) {
	profile := canonicalRuntimeProfile("nds", requestedProfile)
	if profile == "" {
		return input, fmt.Errorf("runtimeProfile is required for Nintendo DS helper uploads")
	}
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(input.Filename))), ".")
	canonical, sourceContainer, err := normalizeNDSUploadPayload(profile, ext, input.Payload)
	if err != nil {
		return input, err
	}
	stem := strings.TrimSpace(strings.TrimSuffix(input.Filename, filepath.Ext(input.Filename)))
	if stem == "" {
		stem = "nds-save"
	}
	capable := true
	input.Payload = canonical
	input.Filename = stem + ".sav"
	input.Format = "sram"
	input.MediaType = "backup-memory"
	input.ProjectionCapable = &capable
	input.SourceArtifactProfile = profile
	input.RuntimeProfile = profile
	input.Metadata = mergeRSMMetadata(input.Metadata, "nds", map[string]any{
		"sourceContainer": sourceContainer,
		"canonicalSize":   len(canonical),
	})
	return input, nil
}

func normalizeNDSUploadPayload(profile, ext string, payload []byte) ([]byte, string, error) {
	profile = canonicalRuntimeProfile("nds", profile)
	switch profile {
	case "nds/desmume":
		if ext == "dsv" {
			raw, err := splitDeSmuMEDSV(payload)
			return raw, "desmume-dsv", err
		}
	case "nds/nogba":
		if raw, ok, err := splitNoGBANDSContainer(payload); ok || err != nil {
			return raw, "nogba-sav", err
		}
	}
	if _, ok := ndsRawSaveSizes[len(payload)]; !ok {
		return nil, "", fmt.Errorf("Nintendo DS raw save size %d is not recognized", len(payload))
	}
	if allBytesEqual(payload, 0x00) || allBytesEqual(payload, 0xFF) {
		return nil, "", fmt.Errorf("Nintendo DS raw save payload is blank")
	}
	return append([]byte(nil), payload...), "raw", nil
}

func projectNDSPayload(summary saveSummary, payload []byte, requestedProfile string) (string, string, []byte, error) {
	profile := canonicalRuntimeProfile("nds", requestedProfile)
	if profile == "" {
		return "", "", nil, fmt.Errorf("unsupported Nintendo DS runtimeProfile %q", requestedProfile)
	}
	canonical, _, err := normalizeStoredNDSPayload(summary, payload)
	if err != nil {
		return "", "", nil, err
	}
	stem := strings.TrimSpace(strings.TrimSuffix(summary.Filename, filepath.Ext(summary.Filename)))
	if stem == "" {
		stem = canonicalSegment(summary.DisplayTitle, "nds-save")
	}
	if stem == "" {
		stem = "nds-save"
	}
	switch profile {
	case "nds/desmume":
		projected, err := buildDeSmuMEDSV(canonical)
		if err != nil {
			return "", "", nil, err
		}
		return safeFilename(stem + ".dsv"), "application/octet-stream", projected, nil
	case "nds/nogba":
		return safeFilename(stem + ".sav"), "application/octet-stream", buildNoGBANDSContainer(canonical), nil
	default:
		definition := runtimeProfilesByID[profile]
		ext := runtimeProfileTargetExtension(summary, definition)
		if ext == "" {
			ext = ".sav"
		}
		return safeFilename(stem + ext), "application/octet-stream", append([]byte(nil), canonical...), nil
	}
}

func normalizeStoredNDSPayload(summary saveSummary, payload []byte) ([]byte, string, error) {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(strings.TrimSpace(summary.Filename))), ".")
	if strings.EqualFold(summary.RuntimeProfile, "nds/desmume") || ext == "dsv" {
		if raw, err := splitDeSmuMEDSV(payload); err == nil {
			return raw, "desmume-dsv", nil
		}
	}
	if strings.EqualFold(summary.RuntimeProfile, "nds/nogba") || ext == "sav" {
		if raw, ok, err := splitNoGBANDSContainer(payload); ok || err != nil {
			return raw, "nogba-sav", err
		}
	}
	if _, ok := ndsRawSaveSizes[len(payload)]; !ok {
		return nil, "", fmt.Errorf("Nintendo DS raw save size %d is not recognized", len(payload))
	}
	return append([]byte(nil), payload...), "raw", nil
}

func splitDeSmuMEDSV(payload []byte) ([]byte, error) {
	// DeSmuME stores the raw backup first and appends a text marker plus a
	// 40-byte binary footer. The raw payload is the only canonical data.
	if len(payload) < desmumeFooterSize {
		return nil, fmt.Errorf("DeSmuME DSV is too small")
	}
	footerStart := len(payload) - desmumeFooterSize
	if !bytes.Equal(payload[footerStart:footerStart+len(desmumeBackupFooterText)], []byte(desmumeBackupFooterText)) {
		return nil, fmt.Errorf("DeSmuME DSV footer text is missing")
	}
	footer := payload[len(payload)-40:]
	if !bytes.Equal(footer[24:40], []byte(desmumeSaveCookie)) {
		return nil, fmt.Errorf("DeSmuME DSV cookie is missing")
	}
	version := binary.LittleEndian.Uint32(footer[20:24])
	if version != 0 {
		return nil, fmt.Errorf("DeSmuME DSV version %d is not supported", version)
	}
	padSize := int(binary.LittleEndian.Uint32(footer[4:8]))
	if padSize != footerStart {
		return nil, fmt.Errorf("DeSmuME DSV backup size %d does not match payload size %d", padSize, footerStart)
	}
	if _, ok := ndsRawSaveSizes[padSize]; !ok {
		return nil, fmt.Errorf("DeSmuME DSV raw size %d is not recognized", padSize)
	}
	return append([]byte(nil), payload[:padSize]...), nil
}

func buildDeSmuMEDSV(canonical []byte) ([]byte, error) {
	saveType, ok := ndsDesmumeSaveTypeForSize(len(canonical))
	if !ok {
		return nil, fmt.Errorf("Nintendo DS raw save size %d is not recognized", len(canonical))
	}
	out := make([]byte, 0, len(canonical)+desmumeFooterSize)
	out = append(out, canonical...)
	out = append(out, []byte(desmumeBackupFooterText)...)
	footer := make([]byte, 40)
	binary.LittleEndian.PutUint32(footer[0:4], uint32(len(canonical)))
	binary.LittleEndian.PutUint32(footer[4:8], uint32(len(canonical)))
	binary.LittleEndian.PutUint32(footer[8:12], saveType.Type)
	binary.LittleEndian.PutUint32(footer[12:16], saveType.AddrSize)
	binary.LittleEndian.PutUint32(footer[16:20], uint32(len(canonical)))
	binary.LittleEndian.PutUint32(footer[20:24], 0)
	copy(footer[24:40], []byte(desmumeSaveCookie))
	out = append(out, footer...)
	return out, nil
}

func ndsDesmumeSaveTypeForSize(size int) (ndsSaveType, bool) {
	saveType, ok := ndsDesmumeSaveTypesBySize[size]
	return saveType, ok
}

func splitNoGBANDSContainer(payload []byte) ([]byte, bool, error) {
	// No$GBA .sav files can be a small header followed by raw SRAM or a compact
	// RLE stream. Returning ok=false lets ordinary raw .sav files flow through.
	if len(payload) < 0x50 {
		return nil, false, nil
	}
	if !bytes.Equal(payload[:len(noGBASaveHeaderID)], []byte(noGBASaveHeaderID)) || payload[0x1F] != noGBASaveHeaderMark {
		return nil, false, nil
	}
	if !bytes.Equal(payload[0x40:0x44], []byte(noGBASaveSRAMID)) {
		return nil, true, fmt.Errorf("No$GBA save header is missing SRAM marker")
	}
	method := binary.LittleEndian.Uint32(payload[0x44:0x48])
	switch method {
	case 0:
		size := int(binary.LittleEndian.Uint32(payload[0x48:0x4C]))
		if size < 0 || 0x4C+size > len(payload) {
			return nil, true, fmt.Errorf("No$GBA uncompressed size %d exceeds payload", size)
		}
		return normalizeNoGBARawOutput(payload[0x4C : 0x4C+size])
	case 1:
		packedSize := int(binary.LittleEndian.Uint32(payload[0x48:0x4C]))
		unpackedSize := int(binary.LittleEndian.Uint32(payload[0x4C:0x50]))
		if packedSize < 0 || 0x50+packedSize > len(payload) {
			return nil, true, fmt.Errorf("No$GBA packed size %d exceeds payload", packedSize)
		}
		raw, err := unpackNoGBARLE(payload[0x50:0x50+packedSize], unpackedSize)
		if err != nil {
			return nil, true, err
		}
		return normalizeNoGBARawOutput(raw)
	default:
		return nil, true, fmt.Errorf("No$GBA compression method %d is not supported", method)
	}
}

func normalizeNoGBARawOutput(raw []byte) ([]byte, bool, error) {
	size := len(raw)
	for size > 0 && raw[size-1] == 0xFF {
		size--
	}
	if size == 0 {
		return nil, true, fmt.Errorf("No$GBA save payload is blank")
	}
	padded, err := padNDSRawSave(raw[:size])
	if err != nil {
		return nil, true, err
	}
	return padded, true, nil
}

func unpackNoGBARLE(packed []byte, unpackedSize int) ([]byte, error) {
	if unpackedSize <= 0 {
		return nil, fmt.Errorf("No$GBA unpacked size is empty")
	}
	out := make([]byte, 0, unpackedSize)
	for pos := 0; pos < len(packed); {
		code := packed[pos]
		pos++
		if code == 0 {
			break
		}
		switch {
		case code == 0x80:
			if pos+3 > len(packed) {
				return nil, fmt.Errorf("No$GBA extended repeat is truncated")
			}
			value := packed[pos]
			count := int(binary.LittleEndian.Uint16(packed[pos+1 : pos+3]))
			pos += 3
			var err error
			out, err = appendRepeatedByte(out, value, count, unpackedSize)
			if err != nil {
				return nil, err
			}
		case code > 0x80:
			if pos >= len(packed) {
				return nil, fmt.Errorf("No$GBA repeat is truncated")
			}
			count := int(code - 0x80)
			value := packed[pos]
			pos++
			var err error
			out, err = appendRepeatedByte(out, value, count, unpackedSize)
			if err != nil {
				return nil, err
			}
		default:
			count := int(code)
			if pos+count > len(packed) {
				return nil, fmt.Errorf("No$GBA literal run is truncated")
			}
			if len(out)+count > unpackedSize {
				return nil, fmt.Errorf("No$GBA unpacked payload exceeds declared size")
			}
			out = append(out, packed[pos:pos+count]...)
			pos += count
		}
	}
	if len(out) != unpackedSize {
		return nil, fmt.Errorf("No$GBA unpacked size %d does not match declared %d", len(out), unpackedSize)
	}
	return out, nil
}

func appendRepeatedByte(out []byte, value byte, count, limit int) ([]byte, error) {
	if len(out)+count > limit {
		return nil, fmt.Errorf("No$GBA unpacked payload exceeds declared size")
	}
	start := len(out)
	out = append(out, make([]byte, count)...)
	for i := start; i < len(out); i++ {
		out[i] = value
	}
	return out, nil
}

func buildNoGBANDSContainer(canonical []byte) []byte {
	out := make([]byte, 0x4C, 0x4C+len(canonical))
	copy(out[:len(noGBASaveHeaderID)], []byte(noGBASaveHeaderID))
	out[0x1F] = noGBASaveHeaderMark
	copy(out[0x40:0x44], []byte(noGBASaveSRAMID))
	binary.LittleEndian.PutUint32(out[0x44:0x48], 0)
	binary.LittleEndian.PutUint32(out[0x48:0x4C], uint32(len(canonical)))
	out = append(out, canonical...)
	return out
}

func padNDSRawSave(payload []byte) ([]byte, error) {
	for _, size := range ndsRawSaveSizeOrder {
		if len(payload) <= size {
			out := bytes.Repeat([]byte{0xFF}, size)
			copy(out, payload)
			return out, nil
		}
	}
	return nil, fmt.Errorf("Nintendo DS raw save size %d is too large", len(payload))
}
