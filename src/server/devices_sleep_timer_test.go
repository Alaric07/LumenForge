package server

import (
	"LumenForge/src/common"
	"LumenForge/src/sleeptimerpresentation"
	"net/http"
	"testing"
)

const devicesSleepTimerTestSerial = "m75-wireless-sleep-test"

type devicesSleepTimerTestTarget struct {
	id       string
	snapshot sleeptimerpresentation.Snapshot
	calls    int
	updated  int
}

func (target *devicesSleepTimerTestTarget) SleepTimerDeviceID() string { return target.id }
func (target *devicesSleepTimerTestTarget) SleepTimerSnapshot() (sleeptimerpresentation.Snapshot, bool) {
	return target.snapshot, true
}
func (target *devicesSleepTimerTestTarget) UpdateSleepTimer(value int) uint8 {
	target.calls++
	target.updated = value
	return 1
}

func TestDevicesSleepTimerRouteValidatesPublishedOptionsBeforeDispatch(t *testing.T) {
	target := &devicesSleepTimerTestTarget{id: devicesSleepTimerTestSerial, snapshot: sleeptimerpresentation.Snapshot{Value: 15, Options: []sleeptimerpresentation.Option{{Value: 1, Label: "1 minute"}, {Value: 15, Label: "15 minutes"}}}}
	previous := lookupDevicesDPIWorkspaceWrapper
	lookupDevicesDPIWorkspaceWrapper = func(serial string) (*common.Device, bool) {
		if serial != devicesSleepTimerTestSerial {
			return nil, false
		}
		return &common.Device{Serial: serial, Instance: target}, true
	}
	t.Cleanup(func() { lookupDevicesDPIWorkspaceWrapper = previous })
	router := setRoutes()
	request := func(body string) *Response {
		target.calls = 0
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/sleep-timer", body)
		return requireLightingMutationResponse(t, recorder, 0)
	}
	for _, body := range []string{
		`{"deviceId":"m75-wireless-sleep-test","sleepTimer":5}`,
		`{"deviceId":"m75-wireless-sleep-test"}`,
		`{"deviceId":"m75-wireless-sleep-test","sleepTimer":15,"extra":true}`,
		`{"deviceId":"unknown","sleepTimer":15}`,
	} {
		if response := request(body); response.Status != 0 || target.calls != 0 {
			t.Fatalf("response=%#v calls=%d", response, target.calls)
		}
	}
	recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/sleep-timer", `{"deviceId":"m75-wireless-sleep-test","sleepTimer":1}`)
	requireLightingMutationResponse(t, recorder, 1)
	if target.calls != 1 || target.updated != 1 {
		t.Fatalf("calls=%d updated=%d", target.calls, target.updated)
	}
}

func TestDevicesSleepTimerSummaryFailsClosedForMalformedOptions(t *testing.T) {
	for _, snapshot := range []sleeptimerpresentation.Snapshot{
		{Value: 1},
		{Value: 1, Options: []sleeptimerpresentation.Option{{Value: 1, Label: ""}}},
		{Value: 1, Options: []sleeptimerpresentation.Option{{Value: 1, Label: "1 minute"}, {Value: 1, Label: "duplicate"}}},
		{Value: 5, Options: []sleeptimerpresentation.Option{{Value: 1, Label: "1 minute"}}},
	} {
		if summary := devicesSleepTimerWorkspaceSummaryFromSnapshot(snapshot); summary != nil {
			t.Fatalf("malformed snapshot produced %#v", summary)
		}
	}
}
