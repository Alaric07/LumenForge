package scimitarrgbelite

import (
	"LumenForge/src/inputmanager"
	"testing"
)

func TestButtonsSnapshotUsesElitePhysicalOrder(t *testing.T) {
	expectedNames := []string{"Right Button", "Middle Button", "Profile Switch", "DPI Button", "Side Button 1", "Side Button 2", "Side Button 3", "Side Button 4", "Side Button 5", "Side Button 6", "Side Button 7", "Side Button 8", "Side Button 9", "Side Button 10", "Side Button 11", "Side Button 12"}
	expectedTypes := []struct {
		id    uint8
		label string
	}{{0, "None"}, {1, "Media Keys"}, {2, "DPI"}, {3, "Keyboard"}, {8, "Sniper"}, {9, "Mouse"}, {10, "Macro"}, {11, "Profile Switch"}, {30, "Cycle Cluster Effect"}}
	d := &Device{
		Serial:             "elite-buttons-test",
		KeyAssignment:      map[int]inputmanager.KeyAssignment{},
		KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile Switch", 30: "Cycle Cluster Effect"},
	}
	for index, key := range scimitarEliteVisibleButtonOrder {
		d.KeyAssignment[key] = inputmanager.KeyAssignment{Name: expectedNames[index], ActionType: 3, ActionCommand: uint16(index)}
	}
	d.KeyAssignment[1024] = inputmanager.KeyAssignment{Name: "Side Button 3", Default: true, ActionHold: true, OnRelease: true, ActionType: 10, ActionCommand: 77, IsMacro: true}
	d.KeyAssignment[1] = inputmanager.KeyAssignment{Name: "Left Button"}

	snapshot, ok := d.ButtonsSnapshot()
	if !ok || len(snapshot.Buttons) != 16 || len(snapshot.AssignmentTypes) != 9 {
		t.Fatalf("ButtonsSnapshot usable/buttons/types = %t/%d/%d", ok, len(snapshot.Buttons), len(snapshot.AssignmentTypes))
	}
	for index, expected := range expectedNames {
		if snapshot.Buttons[index].Name != expected {
			t.Fatalf("button %d = %q, want %q", index, snapshot.Buttons[index].Name, expected)
		}
	}
	for index, expected := range expectedTypes {
		if snapshot.AssignmentTypes[index].ID != expected.id || snapshot.AssignmentTypes[index].Label != expected.label {
			t.Fatalf("assignment type %d = %#v, want %d/%q", index, snapshot.AssignmentTypes[index], expected.id, expected.label)
		}
	}
	state := snapshot.Buttons[6]
	if !state.Default || !state.PressAndHold || !state.OnRelease || state.ActionType != 10 || state.ActionCommand != 77 || !state.IsMacro {
		t.Fatalf("assignment state was not preserved: %#v", state)
	}
	for _, button := range snapshot.Buttons {
		if button.Name == "Left Button" {
			t.Fatal("Left Button was exposed")
		}
	}
}
