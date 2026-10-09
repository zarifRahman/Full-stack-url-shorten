package main

import (
	"strings"
	"testing"
)

func TestShortCode(t *testing.T) {
	c := shortCode(6)
	if len(c) != 6 {
		t.Fatalf("want length 6, got %d (%q)", len(c), c)
	}
	for _, ch := range c {
		if !strings.ContainsRune(alphabet, ch) {
			t.Fatalf("char %q not in alphabet", ch)
		}
	}
	if shortCode(6) == shortCode(6) {
		t.Fatal("two codes collided — effectively impossible, rand broken")
	}
}

func TestNormalizeURL(t *testing.T) {
	cases := map[string]bool{
		"example.com":          true,
		"https://example.com":  true,
		"http://a.b/c?d=e":     true,
		"":                     false,
		"   ":                  false,
	}
	for in, ok := range cases {
		_, err := normalizeURL(in)
		if ok && err != nil {
			t.Errorf("%q: want ok, got err %v", in, err)
		}
		if !ok && err == nil {
			t.Errorf("%q: want err, got ok", in)
		}
	}
}
