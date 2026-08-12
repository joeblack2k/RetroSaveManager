package main

import "testing"

func TestSaveRecordFormat(t *testing.T) {
	cases := []struct {
		name        string
		payloadFile string
		want        string
	}{
		{"gb battery save", "payload.srm", "srm"},
		{"gb rtc save", "payload.rtc", "rtc"},
		{"uppercase extension is normalized", "payload.SRM", "srm"},
		{"no extension", "payload", ""},
		{"empty payload file", "", ""},
		{"multi-dot keeps final ext", "payload.tar.gz", "gz"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := saveRecordFormat(saveRecord{PayloadFile: tc.payloadFile})
			if got != tc.want {
				t.Fatalf("saveRecordFormat(%q) = %q, want %q", tc.payloadFile, got, tc.want)
			}
		})
	}
}
