package main

import "testing"

func TestNormalizeLanguage(t *testing.T) {
	tests := map[string]string{
		"":            "eng",
		"C.UTF-8":     "eng",
		"POSIX":       "eng",
		"en_US.UTF-8": "eng",
		"ru_RU.UTF-8": "rus",
		"uk_UA.UTF-8": "ukr",
		"de_DE.UTF-8": "de",
		"rus":         "rus",
	}

	for input, want := range tests {
		if got := normalizeLanguage(input); got != want {
			t.Errorf("normalizeLanguage(%q) = %q, want %q", input, got, want)
		}
	}
}
