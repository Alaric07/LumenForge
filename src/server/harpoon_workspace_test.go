package server

import (
	"testing"

	"LumenForge/src/common"
	"LumenForge/src/devices/harpoonrgbpro"
	"LumenForge/src/inputmanager"
	"LumenForge/src/rgb"
	"LumenForge/src/stats"
)

func TestHarpoonWorkspaceSummaryUsesSharedMousePresentations(t *testing.T) {
	const serial = "harpoon-modern-workspace"
	assignments := map[int]inputmanager.KeyAssignment{
		1: {Name: "Left Button", Default: true}, 2: {Name: "Right Button", Default: true}, 4: {Name: "Middle Button", Default: true},
		8: {Name: "Back Button", Default: true}, 16: {Name: "Forward Button", Default: true}, 32: {Name: "DPI Button", Default: true},
	}
	instance := &harpoonrgbpro.Device{
		Serial: serial, MinDPI: 200, MaxDPI: 12000,
		DeviceProfile: &harpoonrgbpro.DeviceProfile{Profile: 1, PollingRate: 1, Profiles: map[int]harpoonrgbpro.DPIProfile{
			0: {Name: "Stage 1", Value: 800, Color: &rgb.Color{Red: 255}},
			1: {Name: "Stage 2", Value: 1500, Color: &rgb.Color{Green: 255}},
			5: {Name: "Sniper", Value: 200, Color: &rgb.Color{Blue: 255}, Sniper: true},
		}},
		KeyAssignment:      assignments,
		KeyAssignmentTypes: map[int]string{0: "None", 1: "Media Keys", 2: "DPI", 3: "Keyboard", 8: "Sniper", 9: "Mouse", 10: "Macro", 11: "Profile Switch"},
		PollingRates:       map[int]string{0: "Not Set", 1: "1000 Hz / 1 msec", 2: "500 Hz / 2 msec", 4: "250 Hz / 4 msec", 8: "125 Hz / 8 msec"},
		UserProfiles:       map[string]*harpoonrgbpro.DeviceProfile{"Default": {Active: true}, "FPS": {}},
	}
	device := &common.Device{Serial: serial, Product: "HARPOON RGB PRO", ProductType: common.ProductTypeHarpoonRgbPro, DeviceType: common.DeviceTypeMouse, Instance: instance}
	summary, ok := devicesWorkspaceSummaryForSerial(map[string]*common.Device{serial: device}, map[string]stats.BatteryStats{}, serial)
	if !ok || summary.DPI == nil || summary.Buttons == nil || summary.Performance == nil || summary.Performance.PollingRate == nil || summary.DeviceProfiles == nil || !summary.LegacyLighting {
		t.Fatalf("summary = %#v, ok=%t", summary, ok)
	}
	if summary.Performance.AngleSnapping != nil || summary.Performance.LiftHeight != nil || len(summary.Performance.BooleanSettings) != 0 {
		t.Fatalf("unsupported performance controls = %#v", summary.Performance)
	}
	for _, view := range []struct{ requested, want string }{{"overview", "overview"}, {"dpi", "dpi"}, {"buttons", "buttons"}, {"lighting", "lighting"}} {
		if got := devicesWorkspaceView([]string{view.requested}, summary); got != view.want {
			t.Errorf("view %q = %q, want %q", view.requested, got, view.want)
		}
	}
}
