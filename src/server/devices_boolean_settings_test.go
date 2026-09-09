package server

import (
	"LumenForge/src/booleansettingpresentation"
	"LumenForge/src/common"
	"net/http"
	"testing"
)

const devicesBooleanSettingsTestSerial = "k100-air-boolean-test"

type devicesBooleanSettingsTestTarget struct {
	id       string
	snapshot booleansettingpresentation.Snapshot
	calls    int
	action   string
	value    bool
}

func (target *devicesBooleanSettingsTestTarget) BooleanSettingsDeviceID() string { return target.id }
func (target *devicesBooleanSettingsTestTarget) BooleanSettingsSnapshot() (booleansettingpresentation.Snapshot, bool) {
	return target.snapshot, true
}
func (target *devicesBooleanSettingsTestTarget) UpdateBooleanSetting(action string, value bool) uint8 {
	target.calls++
	target.action, target.value = action, value
	return 1
}

func TestDevicesBooleanSettingRouteValidatesPublishedSettingsBeforeDispatch(t *testing.T) {
	target := &devicesBooleanSettingsTestTarget{id: devicesBooleanSettingsTestSerial, snapshot: booleansettingpresentation.Snapshot{Settings: []booleansettingpresentation.Setting{{ID: "auto-brightness", Label: "Auto Brightness", Action: "auto-brightness"}}}}
	previous := lookupDevicesDPIWorkspaceWrapper
	lookupDevicesDPIWorkspaceWrapper = func(serial string) (*common.Device, bool) {
		if serial != devicesBooleanSettingsTestSerial {
			return nil, false
		}
		return &common.Device{Serial: serial, Instance: target}, true
	}
	t.Cleanup(func() { lookupDevicesDPIWorkspaceWrapper = previous })
	router := setRoutes()
	for _, body := range []string{
		`{"deviceId":"k100-air-boolean-test","settingId":"unknown","value":true}`,
		`{"deviceId":"k100-air-boolean-test","value":true}`,
		`{"deviceId":"k100-air-boolean-test","settingId":"auto-brightness","value":true,"extra":true}`,
		`{"deviceId":"unknown","settingId":"auto-brightness","value":true}`,
	} {
		target.calls = 0
		response := requireLightingMutationResponse(t, requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/boolean-setting", body), 0)
		if response.Status != 0 || target.calls != 0 {
			t.Fatalf("body=%s response=%#v calls=%d", body, response, target.calls)
		}
	}
	requireLightingMutationResponse(t, requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/boolean-setting", `{"deviceId":"k100-air-boolean-test","settingId":"auto-brightness","value":true}`), 1)
	if target.calls != 1 || target.action != "auto-brightness" || !target.value {
		t.Fatalf("calls=%d action=%q value=%t", target.calls, target.action, target.value)
	}
}

func TestDevicesBooleanSettingsSummaryFailsClosed(t *testing.T) {
	for _, snapshot := range []booleansettingpresentation.Snapshot{
		{}, {Settings: []booleansettingpresentation.Setting{{ID: "", Label: "Setting", Action: "action"}}}, {Settings: []booleansettingpresentation.Setting{{ID: "id", Label: "", Action: "action"}}}, {Settings: []booleansettingpresentation.Setting{{ID: "id", Label: "Setting", Action: ""}}}, {Settings: []booleansettingpresentation.Setting{{ID: "id", Label: "First", Action: "one"}, {ID: "id", Label: "Second", Action: "two"}}}, {Settings: []booleansettingpresentation.Setting{{ID: "one", Label: "First", Action: "action"}, {ID: "two", Label: "Second", Action: "action"}}},
	} {
		if summary := devicesBooleanSettingsWorkspaceSummaryFromSnapshot(snapshot); summary != nil {
			t.Fatalf("snapshot=%#v summary=%#v", snapshot, summary)
		}
	}
}
