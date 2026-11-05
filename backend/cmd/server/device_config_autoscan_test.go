package main

import "testing"

// The Steam Machine's stored sources: the configured RetroDECK source and the helper's
// autoscanned steamdeck_default over the same saves folder.
func steamMachineSources() []deviceConfigSource {
	return []deviceConfigSource{
		{ID: "retrodeck", Label: "RetroDeck", Kind: "retroarch", Profile: "retroarch",
			SavePath: "/home/deck/retrodeck/saves", Managed: true, Origin: "backend"},
		{ID: "steamdeck_default", Label: "steamdeck Default", Kind: "steamdeck", Profile: "generic",
			SavePath: "/home/deck/retrodeck/saves/", Managed: true, Origin: "autoscan-known"},
	}
}

func sourceIDs(sources []deviceConfigSource) map[string]bool {
	out := map[string]bool{}
	for _, source := range sources {
		out[source.ID] = true
	}
	return out
}

func TestAutoscanSourceOverAConfiguredSaveFolderIsNotKept(t *testing.T) {
	reported := steamMachineSources()
	merged := mergeHelperReportedConfigSources(reported, reported)
	ids := sourceIDs(merged)
	if !ids["retrodeck"] {
		t.Fatalf("configured source must stay, got %v", ids)
	}
	if ids["steamdeck_default"] {
		t.Fatalf("autoscanned source under the configured save folder must not be kept, got %v", ids)
	}
}

func TestAutoscanSourceInsideAConfiguredSaveFolderIsNotKept(t *testing.T) {
	reported := steamMachineSources()
	reported[1].SavePath = "/home/deck/retrodeck/saves/gb"
	merged := mergeHelperReportedConfigSources(reported, nil)
	if sourceIDs(merged)["steamdeck_default"] {
		t.Fatal("autoscanned source inside the configured save folder must not be kept")
	}
}

func TestAutoscanSourceElsewhereIsKept(t *testing.T) {
	reported := steamMachineSources()
	reported[1].SavePath = "/home/deck/.local/share/other/saves"
	merged := mergeHelperReportedConfigSources(reported, nil)
	if !sourceIDs(merged)["steamdeck_default"] {
		t.Fatal("autoscanned source over a folder no configured source covers must stay")
	}
}

func TestAutoscanSourceComesBackWhenTheConfiguredOneIsGone(t *testing.T) {
	reported := steamMachineSources()[1:]
	merged := mergeHelperReportedConfigSources(reported, nil)
	if !sourceIDs(merged)["steamdeck_default"] {
		t.Fatal("with no configured source covering it, the autoscanned source must stay")
	}
}

func TestAutoscanSourceIsNotSentBackInTheRuntimePolicy(t *testing.T) {
	d := device{DeviceType: "steamdeck", SyncAll: true,
		ConfigSources: mergeHelperReportedConfigSources(steamMachineSources(), nil)}
	for _, item := range runtimeSourcePoliciesForDevice(d) {
		if item["sourceId"] == "steamdeck_default" {
			t.Fatal("the runtime policy must not hand the autoscanned duplicate back to the helper")
		}
	}
}
