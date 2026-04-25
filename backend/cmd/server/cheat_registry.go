package main

import (
	"fmt"
	"sort"
	"strings"
)

var cheatEditorRegistry = map[string]cheatEditor{}

func registerCheatEditor(editor cheatEditor) {
	if editor == nil {
		panic("cannot register nil cheat editor")
	}
	id := strings.TrimSpace(editor.ID())
	if id == "" {
		panic("cannot register cheat editor with empty id")
	}
	if _, exists := cheatEditorRegistry[id]; exists {
		panic(fmt.Sprintf("duplicate cheat editor registration: %s", id))
	}
	cheatEditorRegistry[id] = editor
}

func builtinCheatEditors() map[string]cheatEditor {
	editors := make(map[string]cheatEditor, len(cheatEditorRegistry))
	for id, editor := range cheatEditorRegistry {
		editors[id] = editor
	}
	return editors
}

func builtinCheatEditorIDs() []string {
	ids := make([]string, 0, len(cheatEditorRegistry))
	for id := range cheatEditorRegistry {
		ids = append(ids, id)
	}
	sort.Strings(ids)
	return ids
}
