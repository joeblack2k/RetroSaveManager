package main

import "encoding/json"

type cheatCapability struct {
	Supported      bool   `json:"supported" yaml:"supported"`
	AvailableCount int    `json:"availableCount,omitempty" yaml:"availableCount,omitempty"`
	EditorID       string `json:"editorId,omitempty" yaml:"editorId,omitempty"`
}

type cheatOption struct {
	ID    string `json:"id" yaml:"id"`
	Label string `json:"label" yaml:"label"`
}

type cheatBitOption struct {
	ID    string `json:"id" yaml:"id"`
	Bit   int    `json:"bit" yaml:"bit"`
	Label string `json:"label" yaml:"label"`
}

type cheatOperation struct {
	Kind   string `json:"kind" yaml:"kind"`
	Flag   string `json:"flag,omitempty" yaml:"flag,omitempty"`
	Course string `json:"course,omitempty" yaml:"course,omitempty"`
	Mode   string `json:"mode,omitempty" yaml:"mode,omitempty"`
	Cup    string `json:"cup,omitempty" yaml:"cup,omitempty"`
	Field  string `json:"field,omitempty" yaml:"field,omitempty"`
}

type cheatField struct {
	ID          string           `json:"id" yaml:"id"`
	Label       string           `json:"label" yaml:"label"`
	Description string           `json:"description,omitempty" yaml:"description,omitempty"`
	Type        string           `json:"type" yaml:"type"`
	Min         *int             `json:"min,omitempty" yaml:"min,omitempty"`
	Max         *int             `json:"max,omitempty" yaml:"max,omitempty"`
	Step        *int             `json:"step,omitempty" yaml:"step,omitempty"`
	Options     []cheatOption    `json:"options,omitempty" yaml:"options,omitempty"`
	Bits        []cheatBitOption `json:"bits,omitempty" yaml:"bits,omitempty"`
	BitLabels   []string         `json:"-" yaml:"bitLabels,omitempty"`
	Op          cheatOperation   `json:"op" yaml:"op"`
}

type cheatSection struct {
	ID     string       `json:"id" yaml:"id"`
	Title  string       `json:"title" yaml:"title"`
	Fields []cheatField `json:"fields" yaml:"fields"`
}

type cheatPreset struct {
	ID          string         `json:"id" yaml:"id"`
	Label       string         `json:"label" yaml:"label"`
	Description string         `json:"description,omitempty" yaml:"description,omitempty"`
	Updates     map[string]any `json:"updates,omitempty" yaml:"updates,omitempty"`
}

type cheatSelector struct {
	ID      string        `json:"id" yaml:"id"`
	Label   string        `json:"label" yaml:"label"`
	Type    string        `json:"type" yaml:"type"`
	Options []cheatOption `json:"options,omitempty" yaml:"options,omitempty"`
}

type cheatPackMatch struct {
	TitleAliases []string `json:"titleAliases,omitempty" yaml:"titleAliases,omitempty"`
}

type cheatPackPayload struct {
	ExactSizes []int    `json:"exactSizes,omitempty" yaml:"exactSizes,omitempty"`
	Formats    []string `json:"formats,omitempty" yaml:"formats,omitempty"`
}

type cheatPack struct {
	GameID     string           `json:"gameId" yaml:"gameId"`
	SystemSlug string           `json:"systemSlug" yaml:"systemSlug"`
	EditorID   string           `json:"editorId" yaml:"editorId"`
	Title      string           `json:"title" yaml:"title"`
	Match      cheatPackMatch   `json:"match" yaml:"match"`
	Payload    cheatPackPayload `json:"payload" yaml:"payload"`
	Selector   *cheatSelector   `json:"selector,omitempty" yaml:"selector,omitempty"`
	Sections   []cheatSection   `json:"sections,omitempty" yaml:"sections,omitempty"`
	Presets    []cheatPreset    `json:"presets,omitempty" yaml:"presets,omitempty"`
}

type saveCheatsGetResponse struct {
	Success      bool                 `json:"success"`
	SaveID       string               `json:"saveId"`
	DisplayTitle string               `json:"displayTitle"`
	Cheats       saveCheatEditorState `json:"cheats"`
}

type saveCheatEditorState struct {
	Supported      bool                      `json:"supported"`
	GameID         string                    `json:"gameId,omitempty"`
	SystemSlug     string                    `json:"systemSlug,omitempty"`
	EditorID       string                    `json:"editorId,omitempty"`
	Title          string                    `json:"title,omitempty"`
	AvailableCount int                       `json:"availableCount,omitempty"`
	Selector       *cheatSelector            `json:"selector,omitempty"`
	Sections       []cheatSection            `json:"sections,omitempty"`
	Presets        []cheatPreset             `json:"presets,omitempty"`
	Values         map[string]any            `json:"values,omitempty"`
	SlotValues     map[string]map[string]any `json:"slotValues,omitempty"`
}

type saveCheatApplyRequest struct {
	SaveID    string                     `json:"saveId"`
	EditorID  string                     `json:"editorId"`
	SlotID    string                     `json:"slotId,omitempty"`
	Updates   map[string]json.RawMessage `json:"updates,omitempty"`
	PresetIDs []string                   `json:"presetIds,omitempty"`
}
