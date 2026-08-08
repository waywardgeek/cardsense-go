package hash

import "testing"

func TestPlausibleMatch(t *testing.T) {
	cases := []struct {
		ocr, name string
		want      bool
	}{
		{"Octoprophet", "Octoprophet", true},
		{"Octoprophef", "Octoprophet", true},
		{"Badgermole C", "Badgermole Cub", true},
		{"Badgermole Cub", "Badgermole Cub", true},
		{"Tatyova, Benthic Druid", "Tatyova, Benthic Druid", true},
		{"Wg ZAM", "Merrow Grimeblotter", false},
		{"Wise", "Voyage's End", false},
		{"Rise", "Voyage's End", false},
		{"Wi ae fo", "Merrow Grimeblotter", false},
		{"Vi see  S", "Golgari Thug", false},
		{"Sse AS", "Golgari Thug", false},
	}
	for _, c := range cases {
		if got := plausibleMatch(c.ocr, c.name); got != c.want {
			t.Errorf("plausibleMatch(%q,%q)=%v want %v", c.ocr, c.name, got, c.want)
		}
	}
}
