package server

import (
	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
	"errors"
	"net/http"
	"slices"
	"strings"
	"testing"
)

const authoredZoneTestSerial = "authored-zone-device"

type authoredZoneTargetCall struct {
	effect, scope, zoneID, groupID string
	zoneIDs                        []string
	color                          rgb.Color
}

type authoredZoneTarget struct {
	id        string
	supported bool
	call      authoredZoneTargetCall
	calls     int
	err       error
}

func (t *authoredZoneTarget) LightingDeviceID() string { return t.id }
func (t *authoredZoneTarget) SupportsLightingEffect(effect string) bool {
	return t.supported && effect == "authored"
}
func (t *authoredZoneTarget) SetLightingEffect(string) error    { return nil }
func (t *authoredZoneTarget) SetLightingBrightness(uint8) error { return nil }
func (t *authoredZoneTarget) ResolveLightingEffectSettings(string) (lightingsettings.EffectSettings, error) {
	return lightingsettings.EffectSettings{}, nil
}
func (t *authoredZoneTarget) SetLightingEffectSettings(string, lightingsettings.EffectSettings) error {
	return nil
}
func (t *authoredZoneTarget) ResetLightingEffectSettings(string) error { return nil }
func (t *authoredZoneTarget) SetLightingZoneColor(effect, scope, zoneID, groupID string, color rgb.Color) error {
	t.calls++
	t.call = authoredZoneTargetCall{
		effect:  effect,
		scope:   scope,
		zoneID:  zoneID,
		groupID: groupID,
		color:   color,
	}
	if t.err != nil {
		return t.err
	}
	switch scope {
	case "zone":
		if zoneID == "" || groupID != "" {
			return errors.New("invalid zone")
		}
	case "group":
		if groupID == "" || zoneID != "" {
			return errors.New("invalid group")
		}
	case "all":
		if zoneID != "" || groupID != "" {
			return errors.New("invalid all")
		}
	default:
		return errors.New("invalid scope")
	}
	return nil
}

func (t *authoredZoneTarget) SetLightingZoneColors(effect string, zoneIDs []string, color rgb.Color) error {
	t.calls++
	t.call = authoredZoneTargetCall{effect: effect, scope: "zones", zoneIDs: append([]string(nil), zoneIDs...), color: color}
	if t.err != nil {
		return t.err
	}
	if len(zoneIDs) == 0 {
		return errors.New("empty zones")
	}
	return nil
}

func installAuthoredZoneTarget(t *testing.T, target *authoredZoneTarget, wrapper func() (*common.Device, bool)) {
	t.Helper()
	previous := lookupNativeDeviceLightingWrapper
	lookupNativeDeviceLightingWrapper = func(serial string) (*common.Device, bool) {
		if serial != authoredZoneTestSerial {
			return nil, false
		}
		if wrapper != nil {
			return wrapper()
		}
		return &common.Device{Serial: authoredZoneTestSerial, Instance: target}, true
	}
	t.Cleanup(func() { lookupNativeDeviceLightingWrapper = previous })
}

func TestNativeAuthoredZoneLightingMutation(t *testing.T) {
	target := &authoredZoneTarget{id: authoredZoneTestSerial, supported: true}
	installAuthoredZoneTarget(t, target, nil)
	router := setRoutes()
	request := func(body string) *Response {
		target.calls = 0
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/lighting/zones", body)
		return requireLightingMutationResponse(t, recorder, 0)
	}
	for _, test := range []struct{ name, body string }{
		{"malformed JSON", `{"serial":`},
		{"unknown field", `{"serial":"authored-zone-device","effect":"authored","scope":"all","color":"#010203","extra":true}`},
		{"trailing JSON", `{"serial":"authored-zone-device","effect":"authored","scope":"all","color":"#010203"}{}`},
		{"oversized body", strings.Repeat(" ", openRGBImportRequestLimit+1) + `{"serial":"authored-zone-device","effect":"authored","scope":"all","color":"#010203"}`},
		{"missing serial", `{"effect":"authored","scope":"all","color":"#010203"}`},
		{"invalid serial", `{"serial":"bad serial","effect":"authored","scope":"all","color":"#010203"}`},
		{"device not found", `{"serial":"not-found","effect":"authored","scope":"all","color":"#010203"}`},
		{"unsupported effect", `{"serial":"authored-zone-device","effect":"other","scope":"all","color":"#010203"}`},
		{"invalid scope", `{"serial":"authored-zone-device","effect":"authored","scope":"other","color":"#010203"}`},
		{"missing zone ID", `{"serial":"authored-zone-device","effect":"authored","scope":"zone","color":"#010203"}`},
		{"missing group ID", `{"serial":"authored-zone-device","effect":"authored","scope":"group","color":"#010203"}`},
		{"empty zones", `{"serial":"authored-zone-device","effect":"authored","scope":"zones","zoneIds":[],"color":"#010203"}`},
		{"duplicate zones", `{"serial":"authored-zone-device","effect":"authored","scope":"zones","zoneIds":["1","1"],"color":"#010203"}`},
		{"invalid color", `{"serial":"authored-zone-device","effect":"authored","scope":"all","color":"blue"}`},
	} {
		t.Run(test.name, func(t *testing.T) {
			response := request(test.body)
			if response.Status != 0 || target.calls != 0 {
				t.Fatalf("response = %#v, calls = %d", response, target.calls)
			}
		})
	}
	t.Run("valid multiple zones", func(t *testing.T) {
		target.calls = 0
		recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/lighting/zones", `{"serial":"authored-zone-device","effect":"authored","scope":"zones","zoneIds":["1","4"],"color":"#010203"}`)
		requireLightingMutationResponse(t, recorder, 1)
		if target.calls != 1 || target.call.scope != "zones" || !slices.Equal(target.call.zoneIDs, []string{"1", "4"}) {
			t.Fatalf("call = %#v", target.call)
		}
	})
	for _, test := range []struct{ name, body, scope, zoneID, groupID string }{
		{"zone", `{"serial":"authored-zone-device","effect":"authored","scope":"zone","zoneId":"z-1","color":"#010203"}`, "zone", "z-1", ""},
		{"group", `{"serial":"authored-zone-device","effect":"authored","scope":"group","groupId":"row-1","color":"#010203"}`, "group", "", "row-1"},
		{"all", `{"serial":"authored-zone-device","effect":"authored","scope":"all","color":"#010203"}`, "all", "", ""},
	} {
		t.Run("valid "+test.name, func(t *testing.T) {
			target.calls = 0
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/lighting/zones", test.body)
			requireLightingMutationResponse(t, recorder, 1)
			if target.calls != 1 || target.call.effect != "authored" || target.call.scope != test.scope || target.call.zoneID != test.zoneID || target.call.groupID != test.groupID || target.call.color.Red != 1 || target.call.color.Green != 2 || target.call.color.Blue != 3 {
				t.Fatalf("call = %#v, calls = %d", target.call, target.calls)
			}
		})
	}
	t.Run("target mutation error", func(t *testing.T) {
		target.err = errors.New("fail")
		defer func() { target.err = nil }()
		response := request(`{"serial":"authored-zone-device","effect":"authored","scope":"all","color":"#010203"}`)
		if response.Status != 0 {
			t.Fatalf("response = %#v", response)
		}
	})
}

func TestNativeAuthoredZoneLightingTargetLookupRejectsUnavailableAndMismatchedTargets(t *testing.T) {
	for _, test := range []struct {
		name    string
		wrapper *common.Device
	}{
		{"hidden", &common.Device{Serial: authoredZoneTestSerial, Hidden: true}},
		{"unavailable", &common.Device{Serial: authoredZoneTestSerial, Unavailable: true}},
		{"not authored target", &common.Device{Serial: authoredZoneTestSerial, Instance: &struct{}{}}},
		{"lighting ID mismatch", &common.Device{Serial: authoredZoneTestSerial, Instance: &authoredZoneTarget{id: "other", supported: true}}},
	} {
		t.Run(test.name, func(t *testing.T) {
			installAuthoredZoneTarget(t, nil, func() (*common.Device, bool) { return test.wrapper, true })
			router := setRoutes()
			recorder := requestOpenRGBLightingMutation(t, router, http.MethodPost, "/api/devices/lighting/zones", `{"serial":"authored-zone-device","effect":"authored","scope":"all","color":"#010203"}`)
			requireLightingMutationResponse(t, recorder, 0)
		})
	}
}
