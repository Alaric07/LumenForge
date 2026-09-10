package keyboardassignmentspresentation

import (
	"LumenForge/src/keyboards"
	"testing"
)

func TestBuildSnapshotPreservesUnnamedLightingOnlyKey(t *testing.T) {
	snapshot, ok := BuildSnapshot(Source{
		Profiles:             []string{"Default"},
		ActiveProfile:        "Default",
		KeyboardLayouts:      []string{"US"},
		ActiveKeyboardLayout: "US",
		LayoutClass:          "keyboard-8",
		RowLayoutClass:       "keyboard-row-26",
		Rows: map[int]keyboards.Row{0: {Keys: map[int]keyboards.Key{
			1: {KeyName: "A", Width: 65, Height: 70},
			2: {OnlyColor: true, Width: 81, Height: 20, Left: 10, Top: 5, PacketIndex: []int{414}},
			3: {KeyName: "DIAL", OnlyColor: true, Width: 65, Height: 40},
		}}},
		AssignmentTypes:     map[int]string{0: "None"},
		OmitModifierOptions: true,
	})
	if !ok || len(snapshot.Rows) != 1 || len(snapshot.Rows[0].Keys) != 3 {
		t.Fatalf("snapshot = %#v, ok = %t", snapshot, ok)
	}
	keys := snapshot.Rows[0].Keys
	if !keys[0].Assignable || keys[0].LightingOnly {
		t.Fatalf("real key = %#v", keys[0])
	}
	if keys[1].Assignable || !keys[1].LightingOnly || keys[1].Width != 81 || keys[1].Height != 20 || keys[1].Left != 10 || keys[1].Top != 5 {
		t.Fatalf("lighting-only key = %#v", keys[1])
	}
	if keys[2].Assignable || keys[2].LightingOnly {
		t.Fatalf("named color-only key = %#v", keys[2])
	}
}
