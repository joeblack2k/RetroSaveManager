package main

import (
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"hash/fnv"
	"io/fs"
	"os"
	"path/filepath"
	"sort"
	"strings"
	"time"
	"unicode"
)

const defaultSaveRoot = "./data/saves"

type saveStore struct {
	root string
}

type saveRecord struct {
	Summary     saveSummary `json:"summary"`
	ROMSHA1     string      `json:"romSha1,omitempty"`
	ROMMD5      string      `json:"romMd5,omitempty"`
	SlotName    string      `json:"slotName,omitempty"`
	SystemSlug  string      `json:"systemSlug"`
	GameSlug    string      `json:"gameSlug"`
	PayloadFile string      `json:"payloadFile"`
	payloadPath string      `json:"-"`
	dirPath     string      `json:"-"`
}

type saveCreateInput struct {
	Filename   string
	Payload    []byte
	Game       game
	Format     string
	Metadata   any
	ROMSHA1    string
	ROMMD5     string
	SlotName   string
	SystemSlug string
	GameSlug   string
	CreatedAt  time.Time
}

func newSaveStoreFromEnv() (*saveStore, error) {
	root := os.Getenv("SAVE_ROOT")
	if strings.TrimSpace(root) == "" {
		root = defaultSaveRoot
	}

	absRoot, err := filepath.Abs(root)
	if err != nil {
		return nil, fmt.Errorf("resolve save root: %w", err)
	}
	if err := os.MkdirAll(absRoot, 0o755); err != nil {
		return nil, fmt.Errorf("create save root: %w", err)
	}
	return &saveStore{root: absRoot}, nil
}

func (s *saveStore) isEmpty() (bool, error) {
	records, err := s.load()
	if err != nil {
		return false, err
	}
	return len(records) == 0, nil
}

func (s *saveStore) load() ([]saveRecord, error) {
	records := make([]saveRecord, 0)
	err := filepath.WalkDir(s.root, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() || d.Name() != "metadata.json" {
			return nil
		}

		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		var record saveRecord
		if err := json.Unmarshal(data, &record); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
		record.dirPath = filepath.Dir(path)
		record.payloadPath = filepath.Join(record.dirPath, record.PayloadFile)
		records = append(records, record)
		return nil
	})
	if err != nil {
		return nil, err
	}

	sort.Slice(records, func(i, j int) bool {
		if records[i].Summary.CreatedAt.Equal(records[j].Summary.CreatedAt) {
			return records[i].Summary.ID > records[j].Summary.ID
		}
		return records[i].Summary.CreatedAt.After(records[j].Summary.CreatedAt)
	})
	return records, nil
}

func (s *saveStore) create(input saveCreateInput) (saveRecord, error) {
	if len(input.Payload) == 0 {
		return saveRecord{}, fmt.Errorf("save payload is empty")
	}

	existing, err := s.load()
	if err != nil {
		return saveRecord{}, err
	}

	createdAt := input.CreatedAt.UTC()
	if createdAt.IsZero() {
		createdAt = time.Now().UTC()
	}

	filename := safeFilename(input.Filename)
	format := strings.TrimSpace(input.Format)
	if format == "" {
		format = inferSaveFormat(filename)
	}
	if format == "" {
		format = "unknown"
	}

	slotName := strings.TrimSpace(input.SlotName)
	if slotName == "" {
		slotName = "default"
	}

	systemSlug := canonicalSegment(input.SystemSlug, "unknown-system")
	if systemSlug == "unknown-system" && input.Game.System != nil {
		systemSlug = canonicalSegment(input.Game.System.Slug, "unknown-system")
	}

	gameName := strings.TrimSpace(input.Game.Name)
	if gameName == "" {
		gameName = strings.TrimSuffix(filename, filepath.Ext(filename))
	}
	if strings.TrimSpace(gameName) == "" {
		gameName = "Unknown Game"
	}

	gameSlug := canonicalSegment(input.GameSlug, "")
	if gameSlug == "" {
		gameSlug = canonicalSegment(gameName, "unknown-game")
	}

	if input.Game.ID == 0 {
		input.Game.ID = deterministicGameID(gameName)
	}
	input.Game.Name = gameName

	shaSum := sha256.Sum256(input.Payload)
	shaHex := hex.EncodeToString(shaSum[:])
	version := nextSaveVersion(existing, input, filename)
	id := fmt.Sprintf("save-%d-%s", createdAt.UnixNano(), shaHex[:12])
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".bin"
	}

	targetDir, err := safeJoinUnderRoot(s.root, systemSlug, gameSlug, id)
	if err != nil {
		return saveRecord{}, err
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return saveRecord{}, fmt.Errorf("create save dir: %w", err)
	}

	record := saveRecord{
		Summary: saveSummary{
			ID:        id,
			Game:      input.Game,
			Filename:  filename,
			FileSize:  len(input.Payload),
			Format:    format,
			Version:   version,
			SHA256:    shaHex,
			CreatedAt: createdAt,
			Metadata:  input.Metadata,
		},
		ROMSHA1:     strings.TrimSpace(input.ROMSHA1),
		ROMMD5:      strings.TrimSpace(input.ROMMD5),
		SlotName:    slotName,
		SystemSlug:  systemSlug,
		GameSlug:    gameSlug,
		PayloadFile: "payload" + ext,
		payloadPath: filepath.Join(targetDir, "payload"+ext),
		dirPath:     targetDir,
	}

	payloadPath := filepath.Join(targetDir, record.PayloadFile)
	metadataPath := filepath.Join(targetDir, "metadata.json")
	if err := writeFileAtomic(payloadPath, input.Payload, 0o644); err != nil {
		return saveRecord{}, fmt.Errorf("write payload: %w", err)
	}
	metadataJSON, err := json.MarshalIndent(record, "", "  ")
	if err != nil {
		return saveRecord{}, fmt.Errorf("encode metadata: %w", err)
	}
	if err := writeFileAtomic(metadataPath, metadataJSON, 0o644); err != nil {
		return saveRecord{}, fmt.Errorf("write metadata: %w", err)
	}
	return record, nil
}

func decodeSaveBatchData(data string) ([]byte, error) {
	payload, err := base64.StdEncoding.DecodeString(data)
	if err == nil {
		return payload, nil
	}
	return base64.RawStdEncoding.DecodeString(data)
}

func buildBatchGame(raw json.RawMessage, filename string) game {
	if len(raw) == 0 {
		return fallbackGameFromFilename(filename)
	}

	var envelope struct {
		Type  string          `json:"type"`
		Value json.RawMessage `json:"value"`
	}
	if err := json.Unmarshal(raw, &envelope); err != nil {
		return fallbackGameFromFilename(filename)
	}

	switch envelope.Type {
	case "gameId":
		var id int
		if err := json.Unmarshal(envelope.Value, &id); err == nil && id != 0 {
			return gameForID(id)
		}
	case "name":
		var value struct {
			Name string `json:"name"`
		}
		if err := json.Unmarshal(envelope.Value, &value); err == nil && strings.TrimSpace(value.Name) != "" {
			g := fallbackGameFromFilename(filename)
			g.ID = deterministicGameID(value.Name)
			g.Name = strings.TrimSpace(value.Name)
			if strings.EqualFold(g.Name, "Wario Land II") {
				return gameForID(281)
			}
			return g
		}
	}

	return fallbackGameFromFilename(filename)
}

func fallbackGameFromFilename(filename string) game {
	name := strings.TrimSpace(strings.TrimSuffix(safeFilename(filename), filepath.Ext(filename)))
	if name == "" {
		name = "Unknown Game"
	}
	if strings.EqualFold(name, "Wario Land II") {
		return gameForID(281)
	}
	return game{
		ID:          deterministicGameID(name),
		Name:        name,
		Boxart:      nil,
		BoxartThumb: nil,
		HasParser:   false,
		System:      nil,
	}
}

func gameForID(id int) game {
	if id == 281 {
		gb := &system{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy"}
		return game{ID: 281, Name: "Wario Land II", Boxart: nil, BoxartThumb: nil, HasParser: false, System: gb}
	}
	return game{
		ID:          id,
		Name:        fmt.Sprintf("Game %d", id),
		Boxart:      nil,
		BoxartThumb: nil,
		HasParser:   false,
		System:      nil,
	}
}

func deterministicGameID(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	return int(h.Sum32()%900000) + 1000
}

func nextSaveVersion(existing []saveRecord, input saveCreateInput, filename string) int {
	version := 1
	targetSlot := strings.TrimSpace(input.SlotName)
	if targetSlot == "" {
		targetSlot = "default"
	}
	for _, record := range existing {
		sameTrack := false
		switch {
		case strings.TrimSpace(input.ROMSHA1) != "":
			sameTrack = record.ROMSHA1 == strings.TrimSpace(input.ROMSHA1) && normalizedSlot(record.SlotName) == targetSlot
		case input.Game.ID != 0:
			sameTrack = record.Summary.Game.ID == input.Game.ID
		default:
			sameTrack = record.Summary.Filename == safeFilename(filename)
		}
		if sameTrack && record.Summary.Version >= version {
			version = record.Summary.Version + 1
		}
	}
	return version
}

func normalizedSlot(slot string) string {
	slot = strings.TrimSpace(slot)
	if slot == "" {
		return "default"
	}
	return slot
}

func inferSaveFormat(filename string) string {
	ext := strings.TrimPrefix(strings.ToLower(filepath.Ext(filename)), ".")
	switch ext {
	case "srm", "sav":
		return "sram"
	case "state", "st":
		return "state"
	case "eep", "fla":
		return ext
	default:
		return ext
	}
}

func safeFilename(name string) string {
	name = filepath.Base(strings.TrimSpace(name))
	name = strings.ReplaceAll(name, string(filepath.Separator), "-")
	if name == "." || name == "" {
		return "save.bin"
	}
	return name
}

func canonicalSegment(value, fallback string) string {
	value = strings.TrimSpace(strings.ToLower(value))
	var b strings.Builder
	lastDash := false
	for _, r := range value {
		switch {
		case unicode.IsLetter(r) || unicode.IsDigit(r):
			b.WriteRune(r)
			lastDash = false
		case r == '-' || r == '_' || unicode.IsSpace(r):
			if !lastDash && b.Len() > 0 {
				b.WriteByte('-')
				lastDash = true
			}
		}
	}
	out := strings.Trim(b.String(), "-")
	if out == "" {
		out = fallback
	}
	if out == "" {
		out = "bucket"
	}
	return out
}

func safeJoinUnderRoot(root string, elems ...string) (string, error) {
	joined := filepath.Join(append([]string{root}, elems...)...)
	cleanRoot := filepath.Clean(root)
	cleanJoined := filepath.Clean(joined)
	rel, err := filepath.Rel(cleanRoot, cleanJoined)
	if err != nil {
		return "", fmt.Errorf("relative path: %w", err)
	}
	if rel == ".." || strings.HasPrefix(rel, ".."+string(os.PathSeparator)) {
		return "", fmt.Errorf("path escapes save root")
	}
	return cleanJoined, nil
}

func writeFileAtomic(path string, data []byte, perm fs.FileMode) error {
	dir := filepath.Dir(path)
	tmp, err := os.CreateTemp(dir, ".tmp-save-*")
	if err != nil {
		return err
	}
	tmpName := tmp.Name()
	defer os.Remove(tmpName)

	if _, err := tmp.Write(data); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Chmod(perm); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Sync(); err != nil {
		_ = tmp.Close()
		return err
	}
	if err := tmp.Close(); err != nil {
		return err
	}
	return os.Rename(tmpName, path)
}
