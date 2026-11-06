package main

import "testing"

// A helper that sends a runtimeProfile for one of these systems is refused with
// "runtimeProfile is only valid for projection-capable saves" unless the system
// is in that list. The profiles exist; the capability is what makes them legal.
func TestNewSystemsAreProjectionCapable(t *testing.T) {
	cases := []struct{ slug, profile, ext string }{
		{slug: "ngp", profile: "ngp/mednafen", ext: ".flash"},
		{slug: "virtualboy", profile: "virtualboy/mednafen", ext: ".srm"},
		{slug: "neogeo-cd", profile: "neogeo-cd/retroarch", ext: ".srm"},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			if !isProjectionCapableSystem(tc.slug) {
				t.Fatalf("%s must be projection-capable or its runtimeProfile is refused", tc.slug)
			}
			def, ok := runtimeProfilesByID[tc.profile]
			if !ok {
				t.Fatalf("profile %q is not defined", tc.profile)
			}
			if def.SystemSlug != tc.slug {
				t.Fatalf("profile %q belongs to %q, not %q", tc.profile, def.SystemSlug, tc.slug)
			}
			if def.TargetExtension != tc.ext {
				t.Fatalf("profile %q extension: want %q, got %q", tc.profile, tc.ext, def.TargetExtension)
			}
		})
	}
}

// Being projection-capable is necessary but not sufficient: the upload also has
// to find an adapter path, or it 422s with "runtimeProfile is not supported".
func TestNewSystemsAcceptTheirProjectionUpload(t *testing.T) {
	cases := []struct{ slug, profile string }{
		{slug: "ngp", profile: "ngp/mednafen"},
		{slug: "virtualboy", profile: "virtualboy/mednafen"},
		{slug: "neogeo-cd", profile: "neogeo-cd/retroarch"},
	}
	for _, tc := range cases {
		t.Run(tc.slug, func(t *testing.T) {
			input := saveCreateInput{Filename: "test.srm", SystemSlug: tc.slug}
			if _, err := normalizeProjectionUpload(input, tc.profile); err != nil {
				t.Fatalf("%s %q must have an adapter path: %v", tc.slug, tc.profile, err)
			}
		})
	}
}
