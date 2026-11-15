package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"io"
	"sort"
	"strings"
)

// A PPSSPP save is a folder PSP/SAVEDATA/<DIR>/ holding a data file, PARAM.SFO
// and optional media. Helpers upload it as one flat zip ("<DIR>.pspsave") with
// pspSaveDir=<DIR>, and the server stores that zip as a single record.

const (
	pspSaveDataBundleKind     = "savedata-zip"
	pspSaveDataBundleFormat   = 1
	pspSaveDataBundleParserID = "psp-savedata-bundle"
	pspSaveDataBundleMaxBytes = 64 << 20
	pspParamSFOName           = "PARAM.SFO"
	pspParamSFOHeaderSize     = 20
	pspParamSFOIndexEntrySize = 16
	pspParamSFOFormatInt32    = 0x0404
	pspSaveDirTrackKeyPrefix  = "psp::savedir::"
	pspSaveDataGameIDLength   = 9
)

var pspSaveDataMediaFiles = map[string]struct{}{
	"PARAM.SFO": {},
	"ICON0.PNG": {},
	"ICON1.PMF": {},
	"PIC1.PNG":  {},
	"SND0.AT3":  {},
}

type pspSaveDataFile struct {
	Name   string
	SHA256 string
	Size   int64
}

type pspParamSFO struct {
	Title             string
	SaveDataTitle     string
	SaveDataDirectory string
	SaveDataDetail    string
}

type pspSaveDataBundle struct {
	SaveDir string
	GameID  string
	SFO     pspParamSFO
	Files   []pspSaveDataFile
}

// pspSaveDataBundleFromForm parses the upload as a SAVEDATA bundle when the
// helper declared system=psp and a pspSaveDir. ok is false for every other upload.
func pspSaveDataBundleFromForm(payload []byte, formValue func(string) string) (pspSaveDataBundle, bool, error) {
	if formValue == nil {
		return pspSaveDataBundle{}, false, nil
	}
	saveDir := strings.TrimSpace(formValue("pspSaveDir"))
	if saveDir == "" || safeMultipartSystemSlug(formValue("system"), nil) != "psp" {
		return pspSaveDataBundle{}, false, nil
	}
	bundle, err := parsePSPSaveDataBundle(payload, saveDir)
	return bundle, true, err
}

func parsePSPSaveDataBundle(payload []byte, saveDir string) (pspSaveDataBundle, error) {
	// The folder keeps the case the helper sent: a client restores into a folder of this
	// name, and PPSSPP's are mixed case (ULUS10437GameData00). Comparisons ignore case.
	saveDir = strings.TrimSpace(saveDir)
	if saveDir == "" || !isSafePSPBundleEntryName(saveDir) {
		return pspSaveDataBundle{}, fmt.Errorf("psp save directory %q is not a valid folder name", saveDir)
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return pspSaveDataBundle{}, fmt.Errorf("psp savedata bundle is not a valid zip: %w", err)
	}

	var total uint64
	seen := map[string]struct{}{}
	files := make([]pspSaveDataFile, 0, len(reader.File))
	var sfoBytes []byte
	hasDataFile := false
	for _, entry := range reader.File {
		name := entry.Name
		if !isSafePSPBundleEntryName(name) {
			return pspSaveDataBundle{}, fmt.Errorf("psp savedata bundle entry %q must be a flat file name", name)
		}
		upper := strings.ToUpper(name)
		if _, dup := seen[upper]; dup {
			return pspSaveDataBundle{}, fmt.Errorf("psp savedata bundle has duplicate entry %q", name)
		}
		seen[upper] = struct{}{}
		total += entry.UncompressedSize64
		if total > pspSaveDataBundleMaxBytes {
			return pspSaveDataBundle{}, fmt.Errorf("psp savedata bundle exceeds %d bytes uncompressed", pspSaveDataBundleMaxBytes)
		}
		data, err := readPSPBundleEntry(entry)
		if err != nil {
			return pspSaveDataBundle{}, err
		}
		if uint64(len(data)) != entry.UncompressedSize64 {
			return pspSaveDataBundle{}, fmt.Errorf("psp savedata bundle entry %q size does not match its header", name)
		}
		sum := sha256.Sum256(data)
		files = append(files, pspSaveDataFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))})
		if upper == pspParamSFOName {
			sfoBytes = data
		}
		if _, media := pspSaveDataMediaFiles[upper]; !media {
			hasDataFile = true
		}
	}
	if sfoBytes == nil {
		return pspSaveDataBundle{}, fmt.Errorf("psp savedata bundle is missing PARAM.SFO")
	}
	sfo, err := parseParamSFO(sfoBytes)
	if err != nil {
		return pspSaveDataBundle{}, err
	}
	if !strings.EqualFold(strings.TrimSpace(sfo.SaveDataDirectory), saveDir) {
		return pspSaveDataBundle{}, fmt.Errorf("PARAM.SFO SAVEDATA_DIRECTORY %q does not match save folder %q", sfo.SaveDataDirectory, saveDir)
	}
	if !hasDataFile {
		return pspSaveDataBundle{}, fmt.Errorf("psp savedata bundle has no save data file besides PARAM.SFO and media")
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })

	gameID := strings.ToUpper(saveDir)
	if len(gameID) > pspSaveDataGameIDLength {
		gameID = gameID[:pspSaveDataGameIDLength]
	}
	return pspSaveDataBundle{SaveDir: saveDir, GameID: gameID, SFO: sfo, Files: files}, nil
}

func readPSPBundleEntry(entry *zip.File) ([]byte, error) {
	rc, err := entry.Open()
	if err != nil {
		return nil, fmt.Errorf("open psp savedata bundle entry %q: %w", entry.Name, err)
	}
	defer rc.Close()
	data, err := io.ReadAll(io.LimitReader(rc, pspSaveDataBundleMaxBytes+1))
	if err != nil {
		return nil, fmt.Errorf("read psp savedata bundle entry %q: %w", entry.Name, err)
	}
	if len(data) > pspSaveDataBundleMaxBytes {
		return nil, fmt.Errorf("psp savedata bundle entry %q is too large", entry.Name)
	}
	return data, nil
}

func isSafePSPBundleEntryName(name string) bool {
	if name == "" || name == "." || name == ".." || strings.TrimSpace(name) != name {
		return false
	}
	return !strings.ContainsAny(name, "/\\:\x00") && !strings.Contains(name, "..")
}

// parseParamSFO reads the string fields RetroSaveManager needs from a PSF file.
func parseParamSFO(data []byte) (pspParamSFO, error) {
	if len(data) < pspParamSFOHeaderSize || !bytes.Equal(data[0:4], []byte("\x00PSF")) {
		return pspParamSFO{}, fmt.Errorf("PARAM.SFO has no PSF header")
	}
	keyStart := uint64(binary.LittleEndian.Uint32(data[8:12]))
	dataStart := uint64(binary.LittleEndian.Uint32(data[12:16]))
	count := uint64(binary.LittleEndian.Uint32(data[16:20]))
	size := uint64(len(data))
	if keyStart > size || dataStart > size || pspParamSFOHeaderSize+count*pspParamSFOIndexEntrySize > size {
		return pspParamSFO{}, fmt.Errorf("PARAM.SFO tables are out of bounds")
	}

	values := map[string]string{}
	for i := uint64(0); i < count; i++ {
		row := data[pspParamSFOHeaderSize+i*pspParamSFOIndexEntrySize:]
		keyOffset := keyStart + uint64(binary.LittleEndian.Uint16(row[0:2]))
		format := binary.LittleEndian.Uint16(row[2:4])
		length := uint64(binary.LittleEndian.Uint32(row[4:8]))
		maxLength := uint64(binary.LittleEndian.Uint32(row[8:12]))
		valueOffset := dataStart + uint64(binary.LittleEndian.Uint32(row[12:16]))
		if keyOffset >= size {
			return pspParamSFO{}, fmt.Errorf("PARAM.SFO key %d is out of bounds", i)
		}
		keyEnd := bytes.IndexByte(data[keyOffset:], 0)
		if keyEnd < 0 {
			return pspParamSFO{}, fmt.Errorf("PARAM.SFO key %d is not terminated", i)
		}
		key := string(data[keyOffset : keyOffset+uint64(keyEnd)])
		if format == pspParamSFOFormatInt32 {
			continue
		}
		if maxLength > 0 && length > maxLength {
			length = maxLength
		}
		if valueOffset > size || length > size-valueOffset {
			return pspParamSFO{}, fmt.Errorf("PARAM.SFO value for %s is out of bounds", key)
		}
		values[key] = strings.TrimSpace(strings.TrimRight(string(data[valueOffset:valueOffset+length]), "\x00"))
	}
	return pspParamSFO{
		Title:             values["TITLE"],
		SaveDataTitle:     values["SAVEDATA_TITLE"],
		SaveDataDirectory: values["SAVEDATA_DIRECTORY"],
		SaveDataDetail:    values["SAVEDATA_DETAIL"],
	}, nil
}

func pspUploadMetadata(existing any, bundle pspSaveDataBundle) any {
	files := make([]any, 0, len(bundle.Files))
	for _, file := range bundle.Files {
		files = append(files, map[string]any{"name": file.Name, "sha256": file.SHA256, "size": file.Size})
	}
	return mergeRSMMetadata(existing, "psp", map[string]any{
		"saveDir":   bundle.SaveDir,
		"gameId":    bundle.GameID,
		"bundle":    pspSaveDataBundleKind,
		"format":    pspSaveDataBundleFormat,
		"files":     files,
		"sfoTitle":  bundle.SFO.Title,
		"saveTitle": bundle.SFO.SaveDataTitle,
	})
}

// pspGameFromBundle names the game after the SFO TITLE, falling back to the folder.
func pspGameFromBundle(bundle pspSaveDataBundle) game {
	info := fallbackGameFromFilename(bundle.SaveDir)
	if title := strings.TrimSpace(bundle.SFO.Title); title != "" {
		info.Name = title
		info.DisplayTitle = title
	}
	info.RegionCode = pspRegionFromGameID(bundle.GameID)
	info.RegionFlag = regionFlagFromCode(info.RegionCode)
	info.System = supportedSystemFromSlug("psp")
	return info
}

// pspRegionFromGameID reads the region letter of a PSP product code (ULUS, UCES,
// NPJH, ...) and falls back to the PlayStation product-code prefixes.
func pspRegionFromGameID(gameID string) string {
	code := strings.ToUpper(strings.TrimSpace(gameID))
	if len(code) >= 4 && (strings.HasPrefix(code, "UL") || strings.HasPrefix(code, "UC") || strings.HasPrefix(code, "NP")) {
		switch code[2] {
		case 'U':
			return regionUS
		case 'E':
			return regionEU
		case 'J':
			return regionJP
		}
	}
	return normalizeRegionCode(regionFromProductCode(code))
}

func pspMetadataMap(metadata any) map[string]any {
	root, ok := metadata.(map[string]any)
	if !ok {
		return nil
	}
	rsm, ok := root["rsm"].(map[string]any)
	if !ok {
		return nil
	}
	psp, _ := rsm["psp"].(map[string]any)
	return psp
}

// pspSaveDirFromMetadata returns the uppercased SAVEDATA folder a record belongs to, if any,
// for keys and comparisons.
func pspSaveDirFromMetadata(metadata any) string {
	return strings.ToUpper(pspStoredSaveDir(metadata))
}

// pspStoredSaveDir returns the SAVEDATA folder a record belongs to in the case it was stored.
func pspStoredSaveDir(metadata any) string {
	saveDir, _ := pspMetadataMap(metadata)["saveDir"].(string)
	return strings.TrimSpace(saveDir)
}

func isPSPSaveDataBundleMetadata(metadata any) bool {
	kind, _ := pspMetadataMap(metadata)["bundle"].(string)
	return kind == pspSaveDataBundleKind && pspSaveDirFromMetadata(metadata) != ""
}

// validatePSPSave checks SAVEDATA bundles; loose PSP files keep passing unvalidated.
func validatePSPSave(input saveCreateInput) consoleValidationResult {
	if !isPSPSaveDataBundleMetadata(input.Metadata) {
		return consoleValidationResult{}
	}
	bundle, err := parsePSPSaveDataBundle(input.Payload, pspStoredSaveDir(input.Metadata))
	if err != nil {
		return consoleValidationResult{Rejected: true, RejectReason: err.Error()}
	}
	semantic := map[string]any{
		"saveDir":   bundle.SaveDir,
		"gameId":    bundle.GameID,
		"saveTitle": bundle.SFO.SaveDataTitle,
		"detail":    bundle.SFO.SaveDataDetail,
		"fileCount": len(bundle.Files),
	}
	if region := pspRegionFromGameID(bundle.GameID); region != regionUnknown {
		semantic["region"] = region
	}
	return consoleValidationResult{Inspection: &saveInspection{
		ParserLevel:        saveParserLevelStructural,
		ParserID:           pspSaveDataBundleParserID,
		ValidatedSystem:    "psp",
		ValidatedGameID:    bundle.GameID,
		ValidatedGameTitle: bundle.SFO.Title,
		Evidence: []string{
			"validated PSP SAVEDATA bundle",
			"saveDir=" + bundle.SaveDir,
			fmt.Sprintf("files=%d", len(bundle.Files)),
		},
		PayloadSizeBytes: len(input.Payload),
		SemanticFields:   semantic,
	}}
}
