package main

import "testing"

func TestParseResourceName(t *testing.T) {
	tests := []struct {
		s        string
		resource string
		owner    string
		instance string
	}{
		{"https://git.sr.ht/~emersion/hut", "hut", "~emersion", "git.sr.ht"},
		{"sr.ht/~emersion/hut", "hut", "~emersion", "sr.ht"},
		{"~emersion/hut", "hut", "~emersion", ""},
		{"hut", "hut", "", ""},
	}

	for _, test := range tests {
		resource, owner, instance := parseResourceName(test.s)
		if resource != test.resource {
			t.Errorf("parseResourceName(%q) resource: expected %q, got %q", test.s, test.resource, resource)
		}
		if owner != test.owner {
			t.Errorf("parseResourceName(%q) owner: expected %q, got %q", test.s, test.owner, owner)
		}
		if instance != test.instance {
			t.Errorf("parseResourceName(%q) instance: expected %q, got %q", test.s, test.instance, instance)
		}
	}
}

func TestParseBuildID(t *testing.T) {
	tests := []struct {
		s        string
		id       int32
		instance string
	}{
		{"https://builds.sr.ht/~emersion/job/1", 1, "builds.sr.ht"},
		{"~emersion/job/1", 1, ""},
		{"job/1", 1, ""},
		{"1", 1, ""},
	}

	for _, test := range tests {
		id, instance, err := parseBuildID(test.s)
		if err != nil {
			t.Errorf("parseBuildID(%q) error: %v", test.s, err)
		}
		if id != test.id {
			t.Errorf("parseBuildID(%q) id: expected %d, got %d", test.s, test.id, id)
		}
		if instance != test.instance {
			t.Errorf("parseBuildID(%q) instance: expected %q, got %q", test.s, test.instance, instance)
		}
	}
}
