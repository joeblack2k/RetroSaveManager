package main

import "testing"

func TestKnownSystemsCatalogIncludesNintendoDS(t *testing.T) {
	items := knownSystemsCatalog()
	for _, item := range items {
		if item.Slug != "nds" {
			continue
		}
		if item.Name != "Nintendo DS" {
			t.Fatalf("expected Nintendo DS catalog name, got %q", item.Name)
		}
		if item.Manufacturer != "Nintendo" {
			t.Fatalf("expected Nintendo manufacturer, got %q", item.Manufacturer)
		}
		return
	}
	t.Fatal("expected Nintendo DS in known systems catalog")
}
