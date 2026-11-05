package main

import "testing"

func TestRegionComesOnlyFromTheNamesTags(t *testing.T) {
	for name, want := range map[string]string{
		"Usagi Yojimbo (Japan)": regionJP,
		"Palamedes (Japan)":     regionJP,
		"Palamedes":             regionUnknown,
		"Euro Champ '92":        regionUnknown,
		"Super Mario Bros. Deluxe (USA, Europe) (Rev 1)":                      regionUS,
		"Some Game (Europe)":                                                  regionEU,
		"Super Mario Bros. Deluxe (J) (C)":                                    regionUnknown,
		"Pokemon - Gold Version (USA, Europe) (SGB Enhanced) (GB Compatible)": regionUS,
		"3-ji no Wide Shou - 5 Gatsugou (Japan) (Magazine)":                   regionJP,
	} {
		if _, got, _ := cleanupDisplayTitleRegionAndLanguages(name); got != want {
			t.Errorf("%q: region %s, want %s", name, got, want)
		}
	}
}
