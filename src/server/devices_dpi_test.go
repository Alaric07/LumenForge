package server

import (
	"LumenForge/src/common"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/rgb"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strconv"
	"testing"
)

const devicesDPITestSerial = "scimitar-elite-dpi-test"

type devicesDPITestTarget struct {
	id                  string
	stages              map[int]uint16
	colors              map[int]rgb.Color
	combinedCalls       int
	activeCalls         int
	sniperCalls         int
	snapshot            dpipresentation.Snapshot
}

func (target *devicesDPITestTarget) DPIDeviceID() string { return target.id }
func (target *devicesDPITestTarget) DPISnapshot() (dpipresentation.Snapshot, bool) {
	if len(target.snapshot.Stages) != 0 {
		return target.snapshot, true
	}
	return dpipresentation.Snapshot{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "0", Stages: []dpipresentation.Stage{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#010203"}, {ID: "1", Name: "Sniper", DPI: 400, ColorHex: "#aabbcc", Sniper: true}}}, true
}
func (target *devicesDPITestTarget) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	target.combinedCalls++
	target.stages = stages
	target.colors = colors
	return 1
}

func (target *devicesDPITestTarget) SelectMouseDPIStage(stageID int) uint8 {
	target.activeCalls++
	for index, stage := range target.snapshot.Stages {
		id, err := strconv.Atoi(stage.ID)
		if err == nil && id == stageID && !stage.Sniper {
			target.snapshot.ActiveRegularStageID = stage.ID
			for stageIndex := range target.snapshot.Stages {
				if !target.snapshot.Stages[stageIndex].Sniper {
					target.snapshot.Stages[stageIndex].Active = stageIndex == index
				}
			}
			return 1
		}
	}
	return 0
}

func (target *devicesDPITestTarget) SetMouseSniperMode(active bool) uint8 {
	target.sniperCalls++
	for index := range target.snapshot.Stages {
		if target.snapshot.Stages[index].Sniper {
			target.snapshot.Stages[index].Active = active
			return 1
		}
	}
	return 0
}

func TestDevicesDPIMutationRoute(t *testing.T) {
	target := &devicesDPITestTarget{id: devicesDPITestSerial}
	previous := lookupDevicesDPIWorkspaceWrapper
	lookupDevicesDPIWorkspaceWrapper = func(serial string) (*common.Device, bool) {
		if serial != devicesDPITestSerial { return nil, false }
		return &common.Device{Serial: serial, Instance: target}, true
	}
	t.Cleanup(func() { lookupDevicesDPIWorkspaceWrapper = previous })
	router := setRoutes()
	request := func(body string) *Response {
		target.combinedCalls = 0
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/dpi", body)
		return requireLightingMutationResponse(t, recorder, 0)
	}
	for _, body := range []string{
		`{"serial":"scimitar-elite-dpi-test","stages":[{"id":"0","dpi":99,"color":"#010203"},{"id":"1","dpi":400,"color":"#aabbcc"}]}`,
		`{"serial":"scimitar-elite-dpi-test","stages":[{"id":"0","dpi":800,"color":"blue"},{"id":"1","dpi":400,"color":"#aabbcc"}]}`,
		`{"serial":"scimitar-elite-dpi-test","stages":[{"id":"0","dpi":800,"color":"#010203"},{"id":"0","dpi":400,"color":"#aabbcc"}]}`,
		`{"serial":"ordinary","stages":[]}`,
		`{"serial":"scimitar-elite-dpi-test","stages":[]}`,
		`{"serial":"scimitar-elite-dpi-test","stages":[{"id":"0","dpi":800,"color":"#010203"},{"id":"1","dpi":400,"color":"#aabbcc"}],"extra":true}`,
	} {
		response := request(body)
		if response.Status != 0 || target.combinedCalls != 0 { t.Fatalf("response=%#v calls=%d", response, target.combinedCalls) }
	}
	recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/dpi", `{"serial":"scimitar-elite-dpi-test","stages":[{"id":"0","dpi":1200,"color":"#102030"},{"id":"1","dpi":500,"color":"#aabbcc"}]}`)
	requireLightingMutationResponse(t, recorder, 1)
	if target.combinedCalls != 1 || target.stages[0] != 1200 || target.colors[0].Red != 16 || target.colors[0].Green != 32 || target.colors[0].Blue != 48 { t.Fatalf("dispatch = %#v %#v calls=%d", target.stages, target.colors, target.combinedCalls) }
}

func TestDevicesDPIStatusRoute(t *testing.T) {
	target := &devicesDPITestTarget{id: devicesDPITestSerial}
	wrapper := &common.Device{Serial: devicesDPITestSerial, Instance: target}
	previous := lookupDevicesDPIWorkspaceWrapper
	lookupDevicesDPIWorkspaceWrapper = func(serial string) (*common.Device, bool) {
		if serial != devicesDPITestSerial { return nil, false }
		return wrapper, true
	}
	t.Cleanup(func() { lookupDevicesDPIWorkspaceWrapper = previous })
	router := setRoutes()
	request := func(query string) devicesDPIStatusResponse {
		recorder := httptest.NewRecorder()
		req := httptest.NewRequest(http.MethodGet, "/api/devices/dpi/status"+query, nil)
		addLocalRequestProtection(t, router, req)
		router.ServeHTTP(recorder, req)
		if recorder.Code != http.StatusOK { t.Fatalf("status response HTTP = %d", recorder.Code) }
		var response devicesDPIStatusResponse
		if err := json.NewDecoder(recorder.Body).Decode(&response); err != nil { t.Fatal(err) }
		return response
	}
	target.snapshot = dpipresentation.Snapshot{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "0", Stages: []dpipresentation.Stage{
		{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#010203", Active: true},
		{ID: "1", Name: "Sniper", DPI: 400, ColorHex: "#aabbcc", Sniper: true},
	}}
	if response := request("?serial=" + devicesDPITestSerial); response.Status != 1 || response.ActiveRegularStageID != "0" || response.SniperActive {
		t.Fatalf("regular status response = %#v", response)
	}
	target.snapshot.Stages[1].Active = true
	if response := request("?serial=" + devicesDPITestSerial); response.Status != 1 || response.ActiveRegularStageID != "0" || !response.SniperActive {
		t.Fatalf("Sniper status response = %#v", response)
	}
	for _, test := range []struct {
		name string
		query string
		device *common.Device
	}{
		{name: "invalid serial", query: "?serial=invalid!"},
		{name: "unavailable device", query: "?serial=" + devicesDPITestSerial, device: &common.Device{Serial: devicesDPITestSerial, Instance: target, Unavailable: true}},
		{name: "non DPI device", query: "?serial=" + devicesDPITestSerial, device: &common.Device{Serial: devicesDPITestSerial, Instance: &struct{}{}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			wrapper = test.device
			if test.device == nil { wrapper = &common.Device{Serial: devicesDPITestSerial, Instance: target} }
			if response := request(test.query); response.Status != 0 || response.ActiveRegularStageID != "" || response.SniperActive {
				t.Fatalf("invalid status response = %#v", response)
			}
		})
	}
}

func TestDevicesDPIActiveStageRoute(t *testing.T) {
	target := &devicesDPITestTarget{id: devicesDPITestSerial, snapshot: dpipresentation.Snapshot{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "0", Stages: []dpipresentation.Stage{
		{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#010203", Active: true},
		{ID: "1", Name: "Stage 2", DPI: 1600, ColorHex: "#102030"},
		{ID: "5", Name: "Sniper", DPI: 400, ColorHex: "#aabbcc", Sniper: true},
	}}}
	previous := lookupDevicesDPIWorkspaceWrapper
	lookupDevicesDPIWorkspaceWrapper = func(serial string) (*common.Device, bool) {
		if serial != devicesDPITestSerial { return nil, false }
		return &common.Device{Serial: serial, Instance: target}, true
	}
	t.Cleanup(func() { lookupDevicesDPIWorkspaceWrapper = previous })
	router := setRoutes()
	selectStage := func(body string) *Response {
		target.activeCalls = 0
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/dpi/active", body)
		return requireLightingMutationResponse(t, recorder, 0)
	}
	for _, body := range []string{
		`{"serial":"scimitar-elite-dpi-test","stageId":"5"}`,
		`{"serial":"scimitar-elite-dpi-test","stageId":"9"}`,
		`{"serial":"scimitar-elite-dpi-test","stageId":"1","extra":true}`,
	} {
		if response := selectStage(body); response.Status != 0 || target.activeCalls != 0 { t.Fatalf("invalid selection response=%#v calls=%d", response, target.activeCalls) }
	}
	recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/dpi/active", `{"serial":"scimitar-elite-dpi-test","stageId":"1"}`)
	requireLightingMutationResponse(t, recorder, 1)
	if target.activeCalls != 1 || target.snapshot.ActiveRegularStageID != "1" || !target.snapshot.Stages[1].Active || target.snapshot.Stages[0].Active || target.snapshot.Stages[2].Active {
		t.Fatalf("active stage selection = %#v calls=%d", target.snapshot, target.activeCalls)
	}
	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/api/devices/dpi/status?serial="+devicesDPITestSerial, nil)
	addLocalRequestProtection(t, router, statusRequest)
	router.ServeHTTP(statusRecorder, statusRequest)
	var status devicesDPIStatusResponse
	if err := json.NewDecoder(statusRecorder.Body).Decode(&status); err != nil { t.Fatal(err) }
	if status.Status != 1 || status.ActiveRegularStageID != "1" || status.SniperActive {
		t.Fatalf("status after UI stage selection = %#v", status)
	}
}

func TestDevicesDPISniperRoute(t *testing.T) {
	target := &devicesDPITestTarget{id: devicesDPITestSerial, snapshot: dpipresentation.Snapshot{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "0", Stages: []dpipresentation.Stage{
		{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#010203", Active: true},
		{ID: "1", Name: "Stage 2", DPI: 1600, ColorHex: "#102030"},
		{ID: "5", Name: "Sniper", DPI: 400, ColorHex: "#aabbcc", Sniper: true},
	}}}
	previous := lookupDevicesDPIWorkspaceWrapper
	lookupDevicesDPIWorkspaceWrapper = func(serial string) (*common.Device, bool) {
		if serial != devicesDPITestSerial { return nil, false }
		return &common.Device{Serial: serial, Instance: target}, true
	}
	t.Cleanup(func() { lookupDevicesDPIWorkspaceWrapper = previous })
	router := setRoutes()
	setSniper := func(body string) *Response {
		target.sniperCalls = 0
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/dpi/sniper", body)
		return requireLightingMutationResponse(t, recorder, 0)
	}
	for _, body := range []string{
		`{"serial":"scimitar-elite-dpi-test"}`,
		`{"serial":"scimitar-elite-dpi-test","active":true,"extra":true}`,
		`{"serial":"unavailable","active":true}`,
	} {
		if response := setSniper(body); response.Status != 0 || target.sniperCalls != 0 { t.Fatalf("invalid Sniper response=%#v calls=%d", response, target.sniperCalls) }
	}
	recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/dpi/sniper", `{"serial":"scimitar-elite-dpi-test","active":true}`)
	requireLightingMutationResponse(t, recorder, 1)
	if target.sniperCalls != 1 || target.snapshot.ActiveRegularStageID != "0" || !target.snapshot.Stages[0].Active || !target.snapshot.Stages[2].Active {
		t.Fatalf("Sniper activation = %#v calls=%d", target.snapshot, target.sniperCalls)
	}
	recorder = requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/dpi/active", `{"serial":"scimitar-elite-dpi-test","stageId":"1"}`)
	requireLightingMutationResponse(t, recorder, 1)
	if target.snapshot.ActiveRegularStageID != "1" || !target.snapshot.Stages[1].Active || !target.snapshot.Stages[2].Active {
		t.Fatalf("regular selection while Sniper active = %#v", target.snapshot)
	}
	statusRecorder := httptest.NewRecorder()
	statusRequest := httptest.NewRequest(http.MethodGet, "/api/devices/dpi/status?serial="+devicesDPITestSerial, nil)
	addLocalRequestProtection(t, router, statusRequest)
	router.ServeHTTP(statusRecorder, statusRequest)
	var status devicesDPIStatusResponse
	if err := json.NewDecoder(statusRecorder.Body).Decode(&status); err != nil { t.Fatal(err) }
	if status.Status != 1 || status.ActiveRegularStageID != "1" || !status.SniperActive {
		t.Fatalf("status while Sniper active = %#v", status)
	}
	recorder = requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/dpi/sniper", `{"serial":"scimitar-elite-dpi-test","active":false}`)
	requireLightingMutationResponse(t, recorder, 1)
	if target.snapshot.ActiveRegularStageID != "1" || !target.snapshot.Stages[1].Active || target.snapshot.Stages[2].Active {
		t.Fatalf("Sniper deactivation = %#v", target.snapshot)
	}
}
