package server

import (
	"LumenForge/src/buttonspresentation"
	"LumenForge/src/common"
	"LumenForge/src/stats"
	"LumenForge/src/templates"
	"bytes"
	"strings"
	"testing"
)

type devicesButtonsTestProvider struct {
	serial   string
	snapshot buttonspresentation.Snapshot
}

func (provider devicesButtonsTestProvider) ButtonsDeviceID() string { return provider.serial }
func (provider devicesButtonsTestProvider) ButtonsSnapshot() (buttonspresentation.Snapshot, bool) {
	return provider.snapshot, true
}

func TestDevicesButtonsWorkspacePresentation(t *testing.T) {
	serial := "scimitar-elite-buttons-test"
	names := []string{"Right Button", "Middle Button", "Profile Switch", "DPI Button", "Side Button 1", "Side Button 2", "Side Button 3", "Side Button 4", "Side Button 5", "Side Button 6", "Side Button 7", "Side Button 8", "Side Button 9", "Side Button 10", "Side Button 11", "Side Button 12"}
	types := []buttonspresentation.AssignmentType{{ID: 0, Label: "None"}, {ID: 1, Label: "Media Keys"}, {ID: 2, Label: "DPI"}, {ID: 3, Label: "Keyboard"}, {ID: 8, Label: "Sniper"}, {ID: 9, Label: "Mouse"}, {ID: 10, Label: "Macro"}, {ID: 11, Label: "Profile Switch"}}
	buttons := make([]buttonspresentation.Button, len(names))
	for index, name := range names {
		buttons[index] = buttonspresentation.Button{KeyIndex: index + 2, Name: name, ActionType: 3, ActionCommand: uint16(index + 100)}
	}
	buttons[0].Default = true
	buttons[6].ActionType, buttons[6].ActionCommand = 10, 42
	snapshot := buttonspresentation.Snapshot{Buttons: buttons, AssignmentTypes: types}

	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{
		serial: {Serial: serial, Product: "SCIMITAR RGB ELITE", Instance: devicesButtonsTestProvider{serial: serial, snapshot: snapshot}},
	}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.Buttons == nil {
		t.Fatal("Elite Buttons provider did not expose a workspace")
	}
	if len(summary.Buttons.Buttons) != 16 || len(summary.Buttons.AssignmentTypes) != 8 {
		t.Fatalf("buttons/types = %d/%d, want 16/8", len(summary.Buttons.Buttons), len(summary.Buttons.AssignmentTypes))
	}
	for index, name := range names {
		if summary.Buttons.Buttons[index].Name != name {
			t.Fatalf("button %d = %q, want %q", index, summary.Buttons.Buttons[index].Name, name)
		}
	}
	if summary.Buttons.Buttons[6].ActionType != 10 || summary.Buttons.Buttons[6].ActionCommand != 42 {
		t.Error("existing Macro assignment state did not survive presentation")
	}
	if got := devicesWorkspaceView([]string{"buttons"}, summary); got != "buttons" {
		t.Fatalf("buttons view = %q", got)
	}
	summary.View = "buttons"
	if got := devicesWorkspaceView([]string{"buttons"}, &devicesWorkspaceSummary{}); got != "overview" {
		t.Fatalf("unsupported buttons view = %q", got)
	}

	initializeDevicesPageTestProcess(t)
	var rendered bytes.Buffer
	if err := templates.GetTemplate().ExecuteTemplate(&rendered, "devices.html", templates.Web{Device: summary, Devices: map[string]*common.Device{serial: {Serial: serial}}, BatteryStats: map[string]stats.BatteryStats{}, Page: "devices"}); err != nil {
		t.Fatal(err)
	}
	body := rendered.String()
	if !strings.Contains(body, "view=buttons") || strings.Contains(body, "Left Button") {
		t.Error("Buttons template did not render the expected tab and controls")
	}
	rightButtonRow := body[strings.Index(body, "Right Button"):]
	rightButtonRow = rightButtonRow[:strings.Index(rightButtonRow, "</tr>")]
	if !strings.Contains(rightButtonRow, "data-lf-button-control") || strings.Contains(rightButtonRow, "data-lf-button-control checked") {
		t.Error("backend Default=true did not render as Device ownership")
	}
	middleButtonRow := body[strings.Index(body, "Middle Button"):]
	middleButtonRow = middleButtonRow[:strings.Index(middleButtonRow, "</tr>")]
	if !strings.Contains(middleButtonRow, "data-lf-button-control checked") {
		t.Error("backend Default=false did not render as LumenForge ownership")
	}
}
