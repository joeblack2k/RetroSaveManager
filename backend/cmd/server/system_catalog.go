package main

import (
	"sort"
	"strings"
)

func knownSystemsCatalog() []system {
	return allSupportedSystems()
}

func manufacturerForSystem(slug, name string) string {
	if known := supportedSystemFromSlug(slug); known != nil && strings.TrimSpace(known.Manufacturer) != "" {
		return known.Manufacturer
	}
	if derivedSlug := supportedSystemSlugFromLabel(name); derivedSlug != "" {
		if known := supportedSystemFromSlug(derivedSlug); known != nil && strings.TrimSpace(known.Manufacturer) != "" {
			return known.Manufacturer
		}
	}

	lookup := strings.ToLower(strings.TrimSpace(slug + " " + name))
	switch {
	case strings.Contains(lookup, "nintendo"), strings.Contains(lookup, "gameboy"), strings.Contains(lookup, "game boy"), strings.Contains(lookup, "snes"), strings.Contains(lookup, "n64"), strings.Contains(lookup, "nintendo ds"), strings.Contains(lookup, "nds"):
		return "Nintendo"
	case strings.Contains(lookup, "playstation"), strings.Contains(lookup, "psx"), strings.Contains(lookup, "ps1"), strings.Contains(lookup, "ps2"), strings.Contains(lookup, "ps3"), strings.Contains(lookup, "psp"):
		return "Sony"
	case strings.Contains(lookup, "neogeo"), strings.Contains(lookup, "neo geo"), strings.Contains(lookup, "neo-geo"):
		return "SNK"
	case strings.Contains(lookup, "arcade"), strings.Contains(lookup, "mame"), strings.Contains(lookup, "fbneo"), strings.Contains(lookup, "finalburn"):
		return "Arcade"
	case strings.Contains(lookup, "sega"), strings.Contains(lookup, "genesis"), strings.Contains(lookup, "mega drive"), strings.Contains(lookup, "saturn"), strings.Contains(lookup, "dreamcast"):
		return "Sega"
	case strings.Contains(lookup, "xbox"), strings.Contains(lookup, "microsoft"):
		return "Microsoft"
	case strings.Contains(lookup, "atari"):
		return "Atari"
	default:
		return "Other"
	}
}

func normalizeSystemCatalogEntry(input system) system {
	out := input
	out.Slug = canonicalSegment(out.Slug, "")
	if out.Slug == "" {
		out.Slug = canonicalSegment(out.Name, "unknown-system")
	}
	if strings.TrimSpace(out.Name) == "" {
		out.Name = toDisplayWords(out.Slug)
	}
	if out.ID == 0 {
		out.ID = deterministicSystemID(out.Name)
	}
	out.Manufacturer = strings.TrimSpace(out.Manufacturer)
	if out.Manufacturer == "" {
		out.Manufacturer = manufacturerForSystem(out.Slug, out.Name)
	}
	return out
}

func (a *app) saveSystemsCatalog() []system {
	catalog := make(map[string]system)
	for _, known := range knownSystemsCatalog() {
		normalized := normalizeSystemCatalogEntry(known)
		catalog[normalized.Slug] = normalized
	}

	records := a.snapshotSaveRecords()
	for _, record := range records {
		candidate := system{}
		if record.Summary.Game.System != nil {
			candidate = *record.Summary.Game.System
		}
		if strings.TrimSpace(candidate.Slug) == "" {
			candidate.Slug = record.SystemSlug
		}
		candidate = normalizeSystemCatalogEntry(candidate)
		if candidate.Slug == "" || candidate.Slug == "unknown-system" {
			continue
		}
		if existing, ok := catalog[candidate.Slug]; ok {
			if strings.TrimSpace(existing.Name) == "" || strings.EqualFold(existing.Name, "Unknown System") {
				existing.Name = candidate.Name
			}
			if existing.ID == 0 {
				existing.ID = candidate.ID
			}
			if strings.TrimSpace(existing.Manufacturer) == "" || strings.EqualFold(existing.Manufacturer, "Other") {
				existing.Manufacturer = candidate.Manufacturer
			}
			catalog[candidate.Slug] = normalizeSystemCatalogEntry(existing)
			continue
		}
		catalog[candidate.Slug] = candidate
	}

	items := make([]system, 0, len(catalog))
	for _, entry := range catalog {
		items = append(items, normalizeSystemCatalogEntry(entry))
	}
	if len(items) == 0 {
		items = append(items, knownSystemsCatalog()...)
	}

	sort.Slice(items, func(i, j int) bool {
		if items[i].Manufacturer == items[j].Manufacturer {
			return strings.ToLower(items[i].Name) < strings.ToLower(items[j].Name)
		}
		if items[i].Manufacturer == "Other" {
			return false
		}
		if items[j].Manufacturer == "Other" {
			return true
		}
		return strings.ToLower(items[i].Manufacturer) < strings.ToLower(items[j].Manufacturer)
	})
	return items
}
