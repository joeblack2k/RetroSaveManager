package main

import (
	"fmt"
	"regexp"
	"strings"
	"time"
)

type validationCoverageSummary struct {
	Total         int `json:"total"`
	GameplayFacts int `json:"gameplayFacts"`
	Semantic      int `json:"semantic"`
	Cheats        int `json:"cheats"`
	Missing       int `json:"missing"`
}

type validationCoverageRecord struct {
	SaveID            string    `json:"saveId"`
	DisplayTitle      string    `json:"displayTitle"`
	SystemSlug        string    `json:"systemSlug"`
	SystemName        string    `json:"systemName,omitempty"`
	ParserLevel       string    `json:"parserLevel,omitempty"`
	ParserID          string    `json:"parserId,omitempty"`
	TrustLevel        string    `json:"trustLevel,omitempty"`
	GameplayFactCount int       `json:"gameplayFactCount"`
	HasGameplayFacts  bool      `json:"hasGameplayFacts"`
	CheatsSupported   bool      `json:"cheatsSupported"`
	CheatCount        int       `json:"cheatCount,omitempty"`
	UpdatedAt         time.Time `json:"updatedAt"`
}

func validationGameplayFactCount(fields map[string]any) int {
	count := 0
	for key, value := range fields {
		if !validationLooksLikeGameplayFact(key, value) {
			continue
		}
		count++
	}
	return count
}

func validationLooksLikeGameplayFact(key string, value any) bool {
	if value == nil || strings.TrimSpace(key) == "" {
		return false
	}
	normalized := strings.ToLower(strings.TrimSpace(key))
	if validationTechnicalSemanticKeys[normalized] {
		return false
	}
	if validationTechnicalSemanticPattern.MatchString(normalized) {
		return false
	}
	if _, ok := value.(map[string]any); ok {
		return false
	}
	if items, ok := value.([]any); ok && len(items) > 12 {
		return false
	}
	text := strings.TrimSpace(fmt.Sprint(value))
	return text != "" && len(text) <= 180
}

var validationTechnicalSemanticKeys = map[string]bool{
	"blankcheck": true, "container": true, "copy1magicvalid": true, "copy1nonzerodata": true,
	"copy2magicvalid": true, "copy2nonzerodata": true, "databytespercopy": true,
	"editablefields": true, "embeddedfilename": true, "entrycount": true, "extension": true,
	"format": true, "layout": true, "mediatype": true, "nonffbytes": true, "nonpaddingbytes": true,
	"nonzerobytes": true, "rawsavekind": true, "romlinked": true, "romsha1present": true,
	"semanticdecoderstate": true, "sourcepath": true, "titlecode": true,
	"validbackupslots": true, "validcopies": true, "validprimaryslots": true,
	"verificationlevels": true, "wordswapped": true,
}

var validationTechnicalSemanticPattern = regexp.MustCompile(`(?i)(checksum|crc|magic|offset|bytes|length|payload|source|sha|hash|copy|valid|verified|signature|container|extension|format|media|rom|parser|raw|nonzero|nonff)`)
