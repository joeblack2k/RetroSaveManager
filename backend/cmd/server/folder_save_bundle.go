package main

import (
	"archive/zip"
	"bytes"
	"crypto/sha256"
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"regexp"
	"sort"
	"strings"
	"unicode/utf16"
)

// Some emulators keep a game's save as a folder of files rather than one file:
// Dolphin keeps a Wii title's save in its NAND at title/<hi>/<lo>/data/, and
// Vita3K keeps a Vita title's under ux0/user/00/savedata/<TITLEID>/. A helper
// uploads such a folder as one flat zip with folderSaveDir naming the folder,
// and the server stores that zip as a single record any device can restore,
// the same way a PPSSPP SAVEDATA folder is stored (psp_savedata.go).

const (
	folderSaveBundleKind     = "folder-zip"
	folderSaveBundleFormat   = 1
	folderSaveBundleMaxBytes = 64 << 20
	folderSaveTrackKeyInfix  = "::folder::"
	wiiBannerMagic           = "WIBN"
	wiiBannerTitleOffset     = 0x20
	wiiBannerTitleBytes      = 0x40
)

// The folder each system's bundles are keyed by: a Wii NAND title is its two
// 8-hex-digit halves (00010001/57435645), a Vita save its title ID (PCSE00746).
var folderSaveDirPatterns = map[string]*regexp.Regexp{
	"wii":    regexp.MustCompile(`^[0-9a-f]{8}/[0-9a-f]{8}$`),
	"psvita": regexp.MustCompile(`^[A-Z]{4}[0-9]{5}$`),
}

type folderSaveFile struct {
	Name   string
	SHA256 string
	Size   int64
}

type folderSaveBundle struct {
	System  string
	SaveDir string
	Title   string
	Files   []folderSaveFile
}

// folderSaveBundleFromForm parses the upload as a folder bundle when the helper
// declared folderSaveDir for a system that keeps folder saves. ok is false for
// every other upload, so a Wii data.bin or a loose file is handled as before.
func folderSaveBundleFromForm(payload []byte, formValue func(string) string) (folderSaveBundle, bool, error) {
	if formValue == nil {
		return folderSaveBundle{}, false, nil
	}
	saveDir := strings.TrimSpace(formValue("folderSaveDir"))
	if saveDir == "" {
		return folderSaveBundle{}, false, nil
	}
	system := safeMultipartSystemSlug(formValue("system"), nil)
	if _, known := folderSaveDirPatterns[system]; !known {
		return folderSaveBundle{}, true, fmt.Errorf("system %q does not keep folder saves", system)
	}
	bundle, err := parseFolderSaveBundle(payload, system, saveDir)
	return bundle, true, err
}

func parseFolderSaveBundle(payload []byte, system string, saveDir string) (folderSaveBundle, error) {
	pattern, known := folderSaveDirPatterns[system]
	if !known {
		return folderSaveBundle{}, fmt.Errorf("system %q does not keep folder saves", system)
	}
	if !pattern.MatchString(saveDir) {
		return folderSaveBundle{}, fmt.Errorf("%s save folder %q is not a valid folder id", system, saveDir)
	}
	reader, err := zip.NewReader(bytes.NewReader(payload), int64(len(payload)))
	if err != nil {
		return folderSaveBundle{}, fmt.Errorf("%s folder bundle is not a valid zip: %w", system, err)
	}

	var total uint64
	seen := map[string]struct{}{}
	files := make([]folderSaveFile, 0, len(reader.File))
	var banner []byte
	for _, entry := range reader.File {
		name := entry.Name
		if !isSafeFolderBundleEntryPath(name) {
			return folderSaveBundle{}, fmt.Errorf("%s folder bundle entry %q must be a relative path of plain names", system, name)
		}
		if _, dup := seen[name]; dup {
			return folderSaveBundle{}, fmt.Errorf("%s folder bundle has duplicate entry %q", system, name)
		}
		seen[name] = struct{}{}
		total += entry.UncompressedSize64
		if total > folderSaveBundleMaxBytes {
			return folderSaveBundle{}, fmt.Errorf("%s folder bundle exceeds %d bytes uncompressed", system, folderSaveBundleMaxBytes)
		}
		data, err := readPSPBundleEntry(entry)
		if err != nil {
			return folderSaveBundle{}, err
		}
		if uint64(len(data)) != entry.UncompressedSize64 {
			return folderSaveBundle{}, fmt.Errorf("%s folder bundle entry %q size does not match its header", system, name)
		}
		sum := sha256.Sum256(data)
		files = append(files, folderSaveFile{Name: name, SHA256: hex.EncodeToString(sum[:]), Size: int64(len(data))})
		if system == "wii" && name == "banner.bin" {
			banner = data
		}
	}
	if len(files) == 0 {
		return folderSaveBundle{}, fmt.Errorf("%s folder bundle holds no files", system)
	}
	if isBlankFolderBundle(reader) {
		return folderSaveBundle{}, fmt.Errorf("%s folder bundle is blank: every file is empty or all zero", system)
	}
	sort.Slice(files, func(i, j int) bool { return files[i].Name < files[j].Name })
	return folderSaveBundle{System: system, SaveDir: saveDir, Title: wiiBannerTitle(banner), Files: files}, nil
}

// isSafeFolderBundleEntryPath accepts a file path inside the save folder, such as
// sce_sys/param.sfo: "/"-separated, and every part a plain name (no "..", no
// backslash, colon or NUL, no empty part, so no leading "/" or folder entry).
func isSafeFolderBundleEntryPath(name string) bool {
	if name == "" {
		return false
	}
	for _, part := range strings.Split(name, "/") {
		if !isSafePSPBundleEntryName(part) {
			return false
		}
	}
	return true
}

// isBlankFolderBundle reports a bundle whose every file is empty or all 0x00:
// a folder an emulator created but no game ever wrote.
func isBlankFolderBundle(reader *zip.Reader) bool {
	for _, entry := range reader.File {
		data, err := readPSPBundleEntry(entry)
		if err != nil {
			return false
		}
		for _, b := range data {
			if b != 0 {
				return false
			}
		}
	}
	return true
}

// wiiBannerTitle reads the game title a Wii save's banner.bin carries: UTF-16BE
// after the WIBN header. Empty when there is no banner or it is not one.
func wiiBannerTitle(banner []byte) string {
	if len(banner) < wiiBannerTitleOffset+wiiBannerTitleBytes || string(banner[0:4]) != wiiBannerMagic {
		return ""
	}
	raw := banner[wiiBannerTitleOffset : wiiBannerTitleOffset+wiiBannerTitleBytes]
	units := make([]uint16, 0, len(raw)/2)
	for i := 0; i+1 < len(raw); i += 2 {
		unit := binary.BigEndian.Uint16(raw[i : i+2])
		if unit == 0 {
			break
		}
		units = append(units, unit)
	}
	return strings.TrimSpace(string(utf16.Decode(units)))
}

func folderSaveUploadMetadata(existing any, bundle folderSaveBundle) any {
	files := make([]any, 0, len(bundle.Files))
	for _, file := range bundle.Files {
		files = append(files, map[string]any{"name": file.Name, "sha256": file.SHA256, "size": file.Size})
	}
	return mergeRSMMetadata(existing, "folder", map[string]any{
		"saveDir": bundle.SaveDir,
		"bundle":  folderSaveBundleKind,
		"format":  folderSaveBundleFormat,
		"files":   files,
		"title":   bundle.Title,
	})
}

// folderSaveGameFromBundle names the game after the Wii banner when there is one,
// otherwise after the folder id.
func folderSaveGameFromBundle(bundle folderSaveBundle) game {
	info := fallbackGameFromFilename(strings.ReplaceAll(bundle.SaveDir, "/", "-"))
	if title := strings.TrimSpace(bundle.Title); title != "" {
		info.Name = title
		info.DisplayTitle = title
	}
	info.System = supportedSystemFromSlug(bundle.System)
	return info
}

func folderSaveMetadataMap(metadata any) map[string]any {
	root, ok := metadata.(map[string]any)
	if !ok {
		return nil
	}
	rsm, ok := root["rsm"].(map[string]any)
	if !ok {
		return nil
	}
	folder, _ := rsm["folder"].(map[string]any)
	return folder
}

// folderSaveDirFromMetadata returns the folder a bundle record belongs to, if any.
func folderSaveDirFromMetadata(metadata any) string {
	saveDir, _ := folderSaveMetadataMap(metadata)["saveDir"].(string)
	return strings.TrimSpace(saveDir)
}

func isFolderSaveBundleMetadata(metadata any) bool {
	kind, _ := folderSaveMetadataMap(metadata)["bundle"].(string)
	return kind == folderSaveBundleKind && folderSaveDirFromMetadata(metadata) != ""
}

// folderSaveTrackKey keys a bundle by its folder, so two folders of one game stay
// apart and a bundle never collides with a loose record of the same system.
func folderSaveTrackKey(systemSlug string, saveDir string) string {
	return systemSlug + folderSaveTrackKeyInfix + strings.ToLower(saveDir)
}

// validateFolderSave checks folder bundles; anything else of the system passes on
// to its own validation.
func validateFolderSave(input saveCreateInput, systemSlug string) (consoleValidationResult, bool) {
	if !isFolderSaveBundleMetadata(input.Metadata) {
		return consoleValidationResult{}, false
	}
	bundle, err := parseFolderSaveBundle(input.Payload, systemSlug, folderSaveDirFromMetadata(input.Metadata))
	if err != nil {
		return consoleValidationResult{Rejected: true, RejectReason: err.Error()}, true
	}
	return consoleValidationResult{Inspection: &saveInspection{
		ParserLevel:        saveParserLevelStructural,
		ParserID:           systemSlug + "-folder-bundle",
		ValidatedSystem:    systemSlug,
		ValidatedGameTitle: bundle.Title,
		Evidence: []string{
			"validated " + systemSlug + " folder bundle",
			"saveDir=" + bundle.SaveDir,
			fmt.Sprintf("files=%d", len(bundle.Files)),
		},
		PayloadSizeBytes: len(input.Payload),
		SemanticFields: map[string]any{
			"saveDir":   bundle.SaveDir,
			"fileCount": len(bundle.Files),
		},
	}}, true
}
