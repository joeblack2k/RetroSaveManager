package main

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strings"
	"testing"
)

func TestGameModuleImportAndInspectSave(t *testing.T) {
	service, err := newGameModuleService(t.TempDir())
	if err != nil {
		t.Fatalf("new module service: %v", err)
	}
	zipData := buildTestGameModuleZip(t, "module-game", testModuleWASMResponse(t, map[string]any{
		"supported":          true,
		"parserLevel":        saveParserLevelSemantic,
		"validatedGameTitle": "Module Game",
		"semanticFields": map[string]any{
			"lives": float64(7),
			"stage": "Green Hill",
		},
	}))
	record, err := service.importZip(context.Background(), zipData, gameModuleSourceInfo{Source: gameModuleSourceUploaded, SourcePath: "module-game.rsmodule.zip"})
	if err != nil {
		t.Fatalf("import module: %v", err)
	}
	if record.Manifest.ModuleID != "module-game" || record.Status != gameModuleStatusActive {
		t.Fatalf("unexpected record: %+v", record)
	}
	inspection, ok := service.inspectSave(saveCreateInput{
		Filename:     "Module Game.sav",
		Payload:      []byte{1, 2, 3, 4},
		Format:       "sav",
		SystemSlug:   "gameboy",
		DisplayTitle: "Module Game",
	}, nil)
	if !ok {
		t.Fatal("expected module inspection")
	}
	if inspection.ParserID != "module-game-parser" || inspection.ValidatedGameTitle != "Module Game" {
		t.Fatalf("unexpected inspection: %+v", inspection)
	}
	if got := inspection.SemanticFields["stage"]; got != "Green Hill" {
		t.Fatalf("expected semantic stage, got %+v", inspection.SemanticFields)
	}
}

func TestGameModuleUploadAPIAndCheatCatalog(t *testing.T) {
	h := newContractHarness(t)
	zipData := buildTestGameModuleZip(t, "module-game", testModuleWASMResponse(t, map[string]any{
		"supported": true,
		"values": map[string]any{
			"lives": 7,
		},
		"payload": []byte{9, 8, 7, 6},
		"changed": map[string]any{
			"lives": 9,
		},
	}))
	body, contentType := multipartFileBody(t, "file", "module-game.rsmodule.zip", zipData)
	req := httptest.NewRequest(http.MethodPost, "/api/modules/upload", bytes.NewReader(body))
	req.Header.Set("Content-Type", contentType)
	rr := h.do(req)
	assertStatus(t, rr, http.StatusCreated)
	assertJSONContentType(t, rr)

	list := h.request(http.MethodGet, "/api/modules", nil)
	assertStatus(t, list, http.StatusOK)
	listBody := decodeJSONMap(t, list.Body)
	modules := mustArray(t, listBody["modules"], "modules")
	if len(modules) != 1 {
		t.Fatalf("expected one module: %s", list.Body.String())
	}

	packs := h.request(http.MethodGet, "/api/cheats/packs", nil)
	assertStatus(t, packs, http.StatusOK)
	packItems := mustArray(t, decodeJSONMap(t, packs.Body)["packs"], "packs")
	foundPack := false
	for _, raw := range packItems {
		item := mustObject(t, raw, "pack")
		manifest := mustObject(t, item["manifest"], "pack.manifest")
		if mustString(t, manifest["packId"], "packId") == "gameboy--module-game" {
			foundPack = true
			if mustString(t, manifest["source"], "source") != cheatPackSourceModule {
				t.Fatalf("expected module source: %s", packs.Body.String())
			}
		}
	}
	if !foundPack {
		t.Fatalf("expected module cheat pack: %s", packs.Body.String())
	}

	disabled := h.request(http.MethodPost, "/api/modules/module-game/disable", nil)
	assertStatus(t, disabled, http.StatusOK)
	v1 := h.request(http.MethodGet, "/api/v1/modules", nil)
	assertStatus(t, v1, http.StatusOK)
}

func TestGameModuleImportRejectsUnsafeZip(t *testing.T) {
	service, err := newGameModuleService(t.TempDir())
	if err != nil {
		t.Fatalf("new module service: %v", err)
	}
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	w, err := zw.Create("../rsm-module.yaml")
	if err != nil {
		t.Fatalf("create zip entry: %v", err)
	}
	_, _ = w.Write([]byte("bad"))
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	_, err = service.importZip(context.Background(), buf.Bytes(), gameModuleSourceInfo{Source: gameModuleSourceUploaded})
	if err == nil || !strings.Contains(strings.ToLower(err.Error()), "unsafe") {
		t.Fatalf("expected unsafe path rejection, got %v", err)
	}
}

func TestGameModuleSyncLibraryImportsGitHubModules(t *testing.T) {
	service, err := newGameModuleService(t.TempDir())
	if err != nil {
		t.Fatalf("new module service: %v", err)
	}
	zipData := buildTestGameModuleZip(t, "module-game", testModuleWASMResponse(t, map[string]any{
		"supported": true,
		"values": map[string]any{
			"lives": 7,
		},
	}))
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/repos/example/RetroSaveManager/git/trees/main":
			if r.URL.Query().Get("recursive") != "1" {
				http.Error(w, "recursive query is required", http.StatusBadRequest)
				return
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"tree":[{"path":"modules/module-game.rsmodule.zip","type":"blob"},{"path":"modules/README.md","type":"blob"}]}`))
		case "/raw/example/RetroSaveManager/main/modules/module-game.rsmodule.zip":
			w.Header().Set("Content-Type", "application/octet-stream")
			_, _ = w.Write(zipData)
		default:
			http.NotFound(w, r)
		}
	}))
	defer server.Close()
	t.Setenv("MODULE_LIBRARY_REPO", "example/RetroSaveManager")
	t.Setenv("MODULE_LIBRARY_REF", "main")
	t.Setenv("MODULE_LIBRARY_PATH", "modules")
	t.Setenv("MODULE_LIBRARY_API_BASE", server.URL+"/repos")
	t.Setenv("MODULE_LIBRARY_RAW_BASE", server.URL+"/raw")

	status, err := service.syncLibrary(context.Background())
	if err != nil {
		t.Fatalf("sync library: %v", err)
	}
	if status.ImportedCount != 1 || status.ErrorCount != 0 {
		t.Fatalf("unexpected sync status: %+v", status)
	}
	record, err := service.readModule("module-game")
	if err != nil {
		t.Fatalf("read synced module: %v", err)
	}
	if record.Source != gameModuleSourceGithub || record.SourcePath != "modules/module-game.rsmodule.zip" {
		t.Fatalf("unexpected synced record: %+v", record)
	}
	if _, err := os.Stat(service.moduleWASMPath(record)); err != nil {
		t.Fatalf("synced wasm missing: %v", err)
	}
}

func TestRepositoryGameModulesImport(t *testing.T) {
	root := findRepositoryModuleRoot(t)
	matches, err := filepath.Glob(filepath.Join(root, "*.rsmodule.zip"))
	if err != nil {
		t.Fatalf("glob modules: %v", err)
	}
	if len(matches) == 0 {
		t.Fatalf("expected at least one .rsmodule.zip under %s", root)
	}
	service, err := newGameModuleService(t.TempDir())
	if err != nil {
		t.Fatalf("new module service: %v", err)
	}
	for _, path := range matches {
		data, err := os.ReadFile(path)
		if err != nil {
			t.Fatalf("read %s: %v", path, err)
		}
		record, err := service.importZip(context.Background(), data, gameModuleSourceInfo{Source: gameModuleSourceGithub, SourcePath: filepath.ToSlash(path), SourceSHA256: sha256Hex(data)})
		if err != nil {
			t.Fatalf("import %s: %v", path, err)
		}
		if record.Manifest.ModuleID == "" || record.Status != gameModuleStatusActive {
			t.Fatalf("unexpected record for %s: %+v", path, record)
		}
	}
}

func findRepositoryModuleRoot(t *testing.T) string {
	t.Helper()
	cwd, err := os.Getwd()
	if err != nil {
		t.Fatalf("getwd: %v", err)
	}
	dir := cwd
	for depth := 0; depth < 8; depth++ {
		candidate := filepath.Join(dir, "modules")
		if info, err := os.Stat(candidate); err == nil && info.IsDir() {
			return candidate
		}
		parent := filepath.Dir(dir)
		if parent == dir {
			break
		}
		dir = parent
	}
	t.Fatalf("modules root not found from %s", cwd)
	return ""
}

func buildTestGameModuleZip(t *testing.T, moduleID string, wasm []byte) []byte {
	t.Helper()
	var buf bytes.Buffer
	zw := zip.NewWriter(&buf)
	writeZipFile(t, zw, "rsm-module.yaml", []byte(fmt.Sprintf(`moduleId: %s
schemaVersion: 1
version: 1.0.0
systemSlug: gameboy
gameId: gameboy/module-game
title: Module Game
parserId: %s-parser
wasmFile: parser.wasm
abiVersion: rsm-wasm-json-v1
titleAliases:
  - Module Game
payload:
  exactSizes:
    - 4
  formats:
    - sav
cheatPacks:
  - path: cheats/module-game.yaml
`, moduleID, moduleID)))
	writeZipFile(t, zw, "parser.wasm", wasm)
	writeZipFile(t, zw, "cheats/module-game.yaml", []byte(fmt.Sprintf(`packId: gameboy--module-game
schemaVersion: 1
adapterId: %s-parser
gameId: gameboy/module-game
systemSlug: gameboy
editorId: %s-parser
title: Module Game
match:
  titleAliases:
    - Module Game
payload:
  exactSizes:
    - 4
  formats:
    - sav
sections:
  - id: stats
    title: Stats
    fields:
      - id: lives
        label: Lives
        type: integer
        min: 0
        max: 99
        op: { kind: moduleNumber, field: lives }
`, moduleID, moduleID)))
	if err := zw.Close(); err != nil {
		t.Fatalf("close zip: %v", err)
	}
	return buf.Bytes()
}

func writeZipFile(t *testing.T, zw *zip.Writer, name string, data []byte) {
	t.Helper()
	w, err := zw.Create(name)
	if err != nil {
		t.Fatalf("create zip entry %s: %v", name, err)
	}
	if _, err := w.Write(data); err != nil {
		t.Fatalf("write zip entry %s: %v", name, err)
	}
}

func multipartFileBody(t *testing.T, fieldName, fileName string, data []byte) ([]byte, string) {
	t.Helper()
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)
	part, err := writer.CreateFormFile(fieldName, filepath.Base(fileName))
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := part.Write(data); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	if err := writer.Close(); err != nil {
		t.Fatalf("close multipart: %v", err)
	}
	return body.Bytes(), writer.FormDataContentType()
}

func testModuleWASMResponse(t *testing.T, response map[string]any) []byte {
	t.Helper()
	data, err := json.Marshal(response)
	if err != nil {
		t.Fatalf("marshal wasm response: %v", err)
	}
	return buildConstResponseWASM(t, data)
}

func buildConstResponseWASM(t *testing.T, response []byte) []byte {
	t.Helper()
	const responsePtr = 1024
	ptrLen := (uint64(responsePtr) << 32) | uint64(len(response))
	var out []byte
	out = append(out, 0x00, 0x61, 0x73, 0x6d, 0x01, 0x00, 0x00, 0x00)
	out = appendWASMSection(out, 1, concatBytes(
		uvar(2),
		[]byte{0x60}, uvar(1), []byte{0x7f}, uvar(1), []byte{0x7f},
		[]byte{0x60}, uvar(4), []byte{0x7f, 0x7f, 0x7f, 0x7f}, uvar(1), []byte{0x7e},
	))
	out = appendWASMSection(out, 3, concatBytes(uvar(2), uvar(0), uvar(1)))
	out = appendWASMSection(out, 5, concatBytes(uvar(1), []byte{0x01}, uvar(1), uvar(1)))
	out = appendWASMSection(out, 7, concatBytes(
		uvar(3),
		wasmName("memory"), []byte{0x02}, uvar(0),
		wasmName("rsm_alloc"), []byte{0x00}, uvar(0),
		wasmName("rsm_call"), []byte{0x00}, uvar(1),
	))
	allocBody := concatBytes(uvar(0), []byte{0x41}, svar(4096), []byte{0x0b})
	callBody := concatBytes(uvar(0), []byte{0x42}, svar64(int64(ptrLen)), []byte{0x0b})
	out = appendWASMSection(out, 10, concatBytes(uvar(2), uvar(uint64(len(allocBody))), allocBody, uvar(uint64(len(callBody))), callBody))
	out = appendWASMSection(out, 11, concatBytes(uvar(1), []byte{0x00, 0x41}, svar(responsePtr), []byte{0x0b}, uvar(uint64(len(response))), response))
	return out
}

func appendWASMSection(out []byte, id byte, payload []byte) []byte {
	out = append(out, id)
	out = append(out, uvar(uint64(len(payload)))...)
	out = append(out, payload...)
	return out
}

func concatBytes(parts ...[]byte) []byte {
	var out []byte
	for _, part := range parts {
		out = append(out, part...)
	}
	return out
}

func wasmName(value string) []byte {
	return concatBytes(uvar(uint64(len(value))), []byte(value))
}

func uvar(value uint64) []byte {
	var out []byte
	for {
		b := byte(value & 0x7f)
		value >>= 7
		if value != 0 {
			b |= 0x80
		}
		out = append(out, b)
		if value == 0 {
			return out
		}
	}
}

func svar(value int) []byte { return svar64(int64(value)) }

func svar64(value int64) []byte {
	var out []byte
	more := true
	for more {
		b := byte(value & 0x7f)
		value >>= 7
		signBitSet := b&0x40 != 0
		if (value == 0 && !signBitSet) || (value == -1 && signBitSet) {
			more = false
		} else {
			b |= 0x80
		}
		out = append(out, b)
	}
	return out
}
