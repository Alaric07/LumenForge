package controldialpresentation

import "testing"

func TestValid(t *testing.T) {
	valid := Snapshot{Available: true, Value: 2, Options: []Option{{1, "One"}, {2, "Two"}}}
	if !Valid(valid) {
		t.Fatal("valid rejected")
	}
	for _, s := range []Snapshot{{Available: true}, {Available: true, Value: 1, Options: []Option{{1, ""}}}, {Available: true, Value: 1, Options: []Option{{1, "A"}, {1, "B"}}}, {Available: true, Value: 2, Options: []Option{{1, "A"}}}} {
		if Valid(s) {
			t.Fatalf("invalid accepted %#v", s)
		}
	}
}
