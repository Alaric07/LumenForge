package server

import (
	"LumenForge/src/common"
	"LumenForge/src/dpipresentation"
	"LumenForge/src/rgb"
	"net/http"
	"testing"
)

const devicesDPITestSerial = "scimitar-elite-dpi-test"

type devicesDPITestTarget struct {
	id                  string
	stages              map[int]uint16
	colors              map[int]rgb.Color
	combinedCalls       int
}

func (target *devicesDPITestTarget) DPIDeviceID() string { return target.id }
func (target *devicesDPITestTarget) DPISnapshot() (dpipresentation.Snapshot, bool) {
	return dpipresentation.Snapshot{MinimumDPI: 100, MaximumDPI: 18000, ActiveRegularStageID: "0", Stages: []dpipresentation.Stage{{ID: "0", Name: "Stage 1", DPI: 800, ColorHex: "#010203"}, {ID: "1", Name: "Sniper", DPI: 400, ColorHex: "#aabbcc", Sniper: true}}}, true
}
func (target *devicesDPITestTarget) SaveMouseDPISettings(stages map[int]uint16, colors map[int]rgb.Color) uint8 {
	target.combinedCalls++
	target.stages = stages
	target.colors = colors
	return 1
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
