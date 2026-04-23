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
	SystemPath  string      `json:"systemPath,omitempty"`
	GamePath    string      `json:"gamePath,omitempty"`
	PayloadFile string      `json:"payloadFile"`
	payloadPath string      `json:"-"`
	dirPath     string      `json:"-"`
}

type saveCreateInput struct {
	Filename              string
	Payload               []byte
	Game                  game
	Format                string
	Metadata              any
	ROMSHA1               string
	ROMMD5                string
	SlotName              string
	SystemSlug            string
	GameSlug              string
	SystemPath            string
	GamePath              string
	DisplayTitle          string
	RegionCode            string
	RegionFlag            string
	LanguageCodes         []string
	CoverArtURL           string
	MemoryCard            *memoryCardDetails
	Dreamcast             *dreamcastDetails
	Saturn                *saturnDetails
	Inspection            *saveInspection
	MediaType             string
	ProjectionCapable     *bool
	SourceArtifactProfile string
	TrustedHelperSystem   bool
	RuntimeProfile        string
	CardSlot              string
	ProjectionID          string
	SourceImportID        string
	Portable              *bool
	CreatedAt             time.Time
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
		s.hydrateRecordDerivedFields(&record)
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

	displayTitle := strings.TrimSpace(input.DisplayTitle)
	if displayTitle == "" {
		displayTitle = strings.TrimSpace(input.Game.DisplayTitle)
	}
	if displayTitle == "" {
		displayTitle, _, _ = cleanupDisplayTitleRegionAndLanguages(gameName)
	}
	if displayTitle == "" {
		displayTitle = gameName
	}
	regionCode := normalizeRegionCode(input.RegionCode)
	if regionCode == regionUnknown {
		if normalizeRegionCode(input.Game.RegionCode) != regionUnknown {
			regionCode = normalizeRegionCode(input.Game.RegionCode)
		} else {
			_, detected, _ := cleanupDisplayTitleRegionAndLanguages(gameName)
			regionCode = normalizeRegionCode(detected)
		}
	}
	languageCodes := normalizeLanguageCodes(input.LanguageCodes)
	if len(languageCodes) == 0 {
		languageCodes = normalizeLanguageCodes(input.Game.LanguageCodes)
	}
	if len(languageCodes) == 0 {
		_, _, languageCodes = cleanupDisplayTitleRegionAndLanguages(gameName)
	}
	if len(languageCodes) == 0 {
		_, _, languageCodes = cleanupDisplayTitleRegionAndLanguages(strings.TrimSuffix(filename, filepath.Ext(filename)))
	}
	regionFlag := strings.TrimSpace(input.RegionFlag)
	if regionFlag == "" {
		regionFlag = regionFlagFromCode(regionCode)
	}

	coverArtURL := strings.TrimSpace(input.CoverArtURL)
	if coverArtURL == "" {
		coverArtURL = strings.TrimSpace(input.Game.CoverArtURL)
	}
	if coverArtURL == "" {
		if input.Game.BoxartThumb != nil {
			coverArtURL = strings.TrimSpace(*input.Game.BoxartThumb)
		}
		if coverArtURL == "" && input.Game.Boxart != nil {
			coverArtURL = strings.TrimSpace(*input.Game.Boxart)
		}
	}
	if coverArtURL != "" {
		coverCopy := coverArtURL
		input.Game.CoverArtURL = coverArtURL
		if input.Game.BoxartThumb == nil {
			input.Game.BoxartThumb = &coverCopy
		}
		if input.Game.Boxart == nil {
			boxCopy := coverArtURL
			input.Game.Boxart = &boxCopy
		}
	}
	input.Game.DisplayTitle = displayTitle
	input.Game.RegionCode = regionCode
	input.Game.RegionFlag = regionFlag
	input.Game.LanguageCodes = languageCodes

	shaSum := sha256.Sum256(input.Payload)
	shaHex := hex.EncodeToString(shaSum[:])
	version := nextSaveVersion(existing, input, filename)
	idBase := fmt.Sprintf("save-%d-%s", createdAt.UnixNano(), shaHex[:12])
	ext := filepath.Ext(filename)
	if ext == "" {
		ext = ".bin"
	}

	systemPath := sanitizeDisplayPathSegment(input.SystemPath, "")
	if systemPath == "" && input.Game.System != nil {
		systemPath = sanitizeDisplayPathSegment(input.Game.System.Name, "")
	}
	if systemPath == "" {
		systemPath = sanitizeDisplayPathSegment(systemSlug, "Unknown System")
	}
	gamePath := sanitizeDisplayPathSegment(input.GamePath, "")
	if gamePath == "" {
		gamePath = sanitizeDisplayPathSegment(displayTitle, "")
	}
	if gamePath == "" {
		gamePath = sanitizeDisplayPathSegment(gameSlug, "Unknown Game")
	}

	id := idBase
	targetDir, err := safeJoinUnderRoot(s.root, systemPath, gamePath, id)
	if err != nil {
		return saveRecord{}, err
	}
	for suffix := 2; ; suffix++ {
		if _, statErr := os.Stat(targetDir); statErr != nil {
			if os.IsNotExist(statErr) {
				break
			}
			return saveRecord{}, fmt.Errorf("stat save dir: %w", statErr)
		}
		id = fmt.Sprintf("%s-%d", idBase, suffix)
		targetDir, err = safeJoinUnderRoot(s.root, systemPath, gamePath, id)
		if err != nil {
			return saveRecord{}, err
		}
	}
	if err := os.MkdirAll(targetDir, 0o755); err != nil {
		return saveRecord{}, fmt.Errorf("create save dir: %w", err)
	}

	record := saveRecord{
		Summary: saveSummary{
			ID:                    id,
			Game:                  input.Game,
			DisplayTitle:          displayTitle,
			SystemSlug:            systemSlug,
			RegionCode:            regionCode,
			RegionFlag:            regionFlag,
			LanguageCodes:         languageCodes,
			CoverArtURL:           coverArtURL,
			SaveCount:             1,
			LatestSizeBytes:       len(input.Payload),
			TotalSizeBytes:        len(input.Payload),
			LatestVersion:         version,
			MemoryCard:            input.MemoryCard,
			Dreamcast:             input.Dreamcast,
			Saturn:                input.Saturn,
			Inspection:            input.Inspection,
			MediaType:             strings.TrimSpace(input.MediaType),
			ProjectionCapable:     input.ProjectionCapable,
			SourceArtifactProfile: strings.TrimSpace(input.SourceArtifactProfile),
			RuntimeProfile:        strings.TrimSpace(input.RuntimeProfile),
			CardSlot:              strings.TrimSpace(input.CardSlot),
			ProjectionID:          strings.TrimSpace(input.ProjectionID),
			SourceImportID:        strings.TrimSpace(input.SourceImportID),
			Portable:              input.Portable,
			Filename:              filename,
			FileSize:              len(input.Payload),
			Format:                format,
			Version:               version,
			SHA256:                shaHex,
			CreatedAt:             createdAt,
			Metadata:              input.Metadata,
		},
		ROMSHA1:     strings.TrimSpace(input.ROMSHA1),
		ROMMD5:      strings.TrimSpace(input.ROMMD5),
		SlotName:    slotName,
		SystemSlug:  systemSlug,
		GameSlug:    gameSlug,
		SystemPath:  systemPath,
		GamePath:    gamePath,
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

func (s *saveStore) hydrateRecordDerivedFields(record *saveRecord) {
	if record == nil {
		return
	}

	summary := &record.Summary
	displayTitle, parsedRegion, parsedLanguages := cleanupDisplayTitleRegionAndLanguages(summary.DisplayTitle)
	if displayTitle == "" || displayTitle == "Unknown Game" {
		displayTitle, parsedRegion, parsedLanguages = cleanupDisplayTitleRegionAndLanguages(summary.Game.DisplayTitle)
	}
	if displayTitle == "" || displayTitle == "Unknown Game" {
		displayTitle, parsedRegion, parsedLanguages = cleanupDisplayTitleRegionAndLanguages(summary.Game.Name)
	}
	if displayTitle == "" || displayTitle == "Unknown Game" {
		displayTitle, parsedRegion, parsedLanguages = cleanupDisplayTitleRegionAndLanguages(strings.TrimSuffix(summary.Filename, filepath.Ext(summary.Filename)))
	}
	if strings.TrimSpace(displayTitle) == "" {
		displayTitle = "Unknown Game"
	}
	regionCode := normalizeRegionCode(summary.RegionCode)
	if regionCode == regionUnknown {
		regionCode = normalizeRegionCode(summary.Game.RegionCode)
	}
	if regionCode == regionUnknown {
		regionCode = normalizeRegionCode(parsedRegion)
	}
	if regionCode == regionUnknown {
		_, detected, _ := cleanupDisplayTitleRegionAndLanguages(summary.Filename)
		regionCode = normalizeRegionCode(detected)
	}
	summary.DisplayTitle = displayTitle
	summary.RegionCode = regionCode
	summary.RegionFlag = regionFlagFromCode(regionCode)
	languageCodes := normalizeLanguageCodes(summary.LanguageCodes)
	if len(languageCodes) == 0 {
		languageCodes = normalizeLanguageCodes(summary.Game.LanguageCodes)
	}
	if len(languageCodes) == 0 {
		languageCodes = normalizeLanguageCodes(parsedLanguages)
	}
	if len(languageCodes) == 0 {
		_, _, languageCodes = cleanupDisplayTitleRegionAndLanguages(summary.Filename)
	}
	summary.LanguageCodes = languageCodes

	cover := strings.TrimSpace(summary.CoverArtURL)
	if cover == "" {
		cover = strings.TrimSpace(summary.Game.CoverArtURL)
	}
	if cover == "" && summary.Game.BoxartThumb != nil {
		cover = strings.TrimSpace(*summary.Game.BoxartThumb)
	}
	if cover == "" && summary.Game.Boxart != nil {
		cover = strings.TrimSpace(*summary.Game.Boxart)
	}
	summary.CoverArtURL = cover
	if cover != "" {
		coverCopy := cover
		summary.Game.CoverArtURL = cover
		if summary.Game.BoxartThumb == nil {
			summary.Game.BoxartThumb = &coverCopy
		}
		if summary.Game.Boxart == nil {
			boxCopy := cover
			summary.Game.Boxart = &boxCopy
		}
	}

	summary.Game.DisplayTitle = summary.DisplayTitle
	summary.Game.Name = summary.DisplayTitle
	summary.Game.RegionCode = summary.RegionCode
	summary.Game.RegionFlag = summary.RegionFlag
	summary.Game.LanguageCodes = summary.LanguageCodes

	if summary.SaveCount <= 0 {
		summary.SaveCount = 1
	}
	if summary.LatestSizeBytes <= 0 {
		summary.LatestSizeBytes = summary.FileSize
	}
	if summary.TotalSizeBytes <= 0 {
		summary.TotalSizeBytes = summary.FileSize
	}
	if summary.LatestVersion <= 0 {
		summary.LatestVersion = summary.Version
	}

	if strings.TrimSpace(record.SystemSlug) == "" {
		if summary.Game.System != nil {
			record.SystemSlug = canonicalSegment(summary.Game.System.Slug, "")
			if record.SystemSlug == "" {
				record.SystemSlug = canonicalSegment(summary.Game.System.Name, "unknown-system")
			}
		}
		if record.SystemSlug == "" {
			record.SystemSlug = "unknown-system"
		}
	}
	summary.SystemSlug = record.SystemSlug
	if summary.Game.System == nil && isSupportedSystemSlug(summary.SystemSlug) {
		summary.Game.System = supportedSystemFromSlug(summary.SystemSlug)
	}
	if summary.Game.System != nil {
		if slug := supportedSystemSlugFromLabel(firstNonEmpty(summary.Game.System.Slug, summary.Game.System.Name)); slug != "" {
			supported := supportedSystemFromSlug(slug)
			summary.Game.System = supported
			record.SystemSlug = supported.Slug
			summary.SystemSlug = supported.Slug
		}
		if strings.TrimSpace(summary.Game.System.Slug) == "" {
			summary.Game.System.Slug = summary.SystemSlug
		}
		if strings.TrimSpace(summary.Game.System.Manufacturer) == "" {
			summary.Game.System.Manufacturer = manufacturerForSystem(summary.Game.System.Slug, summary.Game.System.Name)
		}
	}
	if strings.TrimSpace(record.GameSlug) == "" {
		record.GameSlug = canonicalGameSlugForTrack(canonicalTrackFromRecord(*record))
	}

	if strings.TrimSpace(record.SystemPath) == "" {
		systemName := record.SystemSlug
		if summary.Game.System != nil && strings.TrimSpace(summary.Game.System.Name) != "" {
			systemName = summary.Game.System.Name
		}
		record.SystemPath = sanitizeDisplayPathSegment(systemName, "Unknown System")
	}
	if strings.TrimSpace(record.GamePath) == "" {
		record.GamePath = sanitizeDisplayPathSegment(summary.DisplayTitle, "Unknown Game")
	}

	if payload, err := os.ReadFile(record.payloadPath); err == nil {
		artifactKind := classifyPlayStationArtifact(summary.Game.System, summary.Format, summary.Filename, payload)
		if artifactKind == saveArtifactPS1MemoryCard || artifactKind == saveArtifactPS2MemoryCard {
			cardName := canonicalMemoryCardName(summary.MemoryCard, record.SlotName, summary.Filename)
			summary.MemoryCard = parsePlayStationMemoryCard(summary.Game.System, payload, summary.Filename, cardName)
			if summary.MemoryCard == nil {
				summary.MemoryCard = &memoryCardDetails{Name: cardName}
			}
			summary.DisplayTitle = summary.MemoryCard.Name
			summary.Game.DisplayTitle = summary.DisplayTitle
			summary.Game.Name = summary.DisplayTitle
			record.GamePath = sanitizeDisplayPathSegment(summary.DisplayTitle, "Memory Card 1")
		} else {
			summary.MemoryCard = nil
		}
		if supportedSystemSlugFromLabel(firstNonEmpty(summary.SystemSlug, record.SystemSlug)) == "saturn" {
			if parsed := parseSaturnContainer(summary.Filename, payload); parsed != nil {
				summary.Saturn = parsed.Details
			} else {
				summary.Saturn = nil
			}
		}
	}

	track := canonicalTrackFromRecord(*record)
	summary.DisplayTitle = track.DisplayTitle
	summary.RegionCode = canonicalRegion(summary.RegionCode, track.RegionCode)
	summary.RegionFlag = regionFlagFromCode(summary.RegionCode)
	summary.Game.ID = canonicalGameIDForTrack(track)
	summary.Game.Name = track.DisplayTitle
	summary.Game.DisplayTitle = track.DisplayTitle
	summary.Game.RegionCode = summary.RegionCode
	summary.Game.RegionFlag = summary.RegionFlag
	record.GameSlug = canonicalGameSlugForTrack(track)
	record.SystemPath = sanitizeDisplayPathSegment(func() string {
		if track.System != nil {
			return track.System.Name
		}
		return record.SystemSlug
	}(), "Unknown System")
	record.GamePath = sanitizeDisplayPathSegment(track.DisplayTitle, "Unknown Game")
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
			g.DisplayTitle = strings.TrimSpace(value.Name)
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
	displayTitle, regionCode, languageCodes := cleanupDisplayTitleRegionAndLanguages(name)
	return game{
		ID:            deterministicGameID(name),
		Name:          displayTitle,
		DisplayTitle:  displayTitle,
		RegionCode:    regionCode,
		RegionFlag:    regionFlagFromCode(regionCode),
		LanguageCodes: languageCodes,
		Boxart:        nil,
		BoxartThumb:   nil,
		HasParser:     false,
		System:        nil,
	}
}

func gameForID(id int) game {
	if id == 281 {
		gb := &system{ID: 19, Name: "Nintendo Game Boy", Slug: "gameboy"}
		return game{
			ID:            281,
			Name:          "Wario Land II",
			DisplayTitle:  "Wario Land II",
			RegionCode:    regionUnknown,
			RegionFlag:    regionFlagFromCode(regionUnknown),
			LanguageCodes: nil,
			Boxart:        nil,
			BoxartThumb:   nil,
			HasParser:     false,
			System:        gb,
		}
	}
	return game{
		ID:            id,
		Name:          fmt.Sprintf("Game %d", id),
		DisplayTitle:  fmt.Sprintf("Game %d", id),
		RegionCode:    regionUnknown,
		RegionFlag:    regionFlagFromCode(regionUnknown),
		LanguageCodes: nil,
		Boxart:        nil,
		BoxartThumb:   nil,
		HasParser:     false,
		System:        nil,
	}
}

func deterministicGameID(name string) int {
	h := fnv.New32a()
	_, _ = h.Write([]byte(strings.ToLower(strings.TrimSpace(name))))
	return int(h.Sum32()%900000) + 1000
}

func nextSaveVersion(existing []saveRecord, input saveCreateInput, filename string) int {
	version := 1
	targetKey := canonicalVersionKeyForInput(input, filename)
	for _, record := range existing {
		if canonicalVersionKeyForRecord(record) == targetKey && record.Summary.Version >= version {
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
