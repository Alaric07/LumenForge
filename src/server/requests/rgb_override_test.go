package requests

import (
	"bytes"
	"net/http/httptest"
	"reflect"
	"testing"

	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/language"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
)

const rgbOverrideRequestSerial = "openrgb-override-request-test"

type rgbOverrideMethodCall struct {
	method string
	args   []interface{}
}

type rgbOverrideRequestSeams struct {
	lookup func(string) (*common.Device, *openrgbimport.Device, bool)
	get    func(string) interface{}
	call   func(string, string, ...interface{}) []reflect.Value
}

func installRGBOverrideRequestSeams(t *testing.T) *rgbOverrideRequestSeams {
	t.Helper()
	previousLookup := lookupOpenRGBOverrideDevice
	previousGet := getRGBOverrideDevice
	previousCall := callRGBOverrideDeviceMethod
	seams := &rgbOverrideRequestSeams{
		lookup: func(string) (*common.Device, *openrgbimport.Device, bool) { return nil, nil, false },
		get:    func(string) interface{} { return nil },
		call:   func(string, string, ...interface{}) []reflect.Value { return nil },
	}
	lookupOpenRGBOverrideDevice = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
		return seams.lookup(serial)
	}
	getRGBOverrideDevice = func(serial string) interface{} { return seams.get(serial) }
	callRGBOverrideDeviceMethod = func(serial, method string, args ...interface{}) []reflect.Value {
		return seams.call(serial, method, args...)
	}
	t.Cleanup(func() {
		lookupOpenRGBOverrideDevice = previousLookup
		getRGBOverrideDevice = previousGet
		callRGBOverrideDeviceMethod = previousCall
	})
	return seams
}

func rgbOverrideSetRequest(body string) *Payload {
	request := httptest.NewRequest("POST", "/api/color/setOverride", bytes.NewBufferString(body))
	return ProcessSetRgbOverride(request)
}

func rgbOverrideGetRequest(body string) *Payload {
	request := httptest.NewRequest("POST", "/api/color/getOverride", bytes.NewBufferString(body))
	return ProcessGetRgbOverride(request)
}

func installImportedRGBOverrideTarget(t *testing.T, seams *rgbOverrideRequestSeams) {
	t.Helper()
	device := &openrgbimport.Device{Serial: rgbOverrideRequestSerial, IsOpenRGB: true}
	wrapper := &common.Device{Serial: rgbOverrideRequestSerial, Instance: device}
	seams.lookup = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
		if serial != rgbOverrideRequestSerial {
			t.Fatalf("imported RGB Override lookup serial = %q, want %q", serial, rgbOverrideRequestSerial)
		}
		return wrapper, device, true
	}
	seams.get = func(string) interface{} {
		panic("imported RGB Override request fell through to native lookup")
	}
	seams.call = func(string, string, ...interface{}) []reflect.Value {
		panic("imported RGB Override request fell through to native reflection")
	}
}

func TestOpenRGBOverrideRequestsAreRejectedWithoutNativeFallback(t *testing.T) {
	t.Run("get", func(t *testing.T) {
		seams := installRGBOverrideRequestSeams(t)
		installImportedRGBOverrideTarget(t, seams)
		response := rgbOverrideGetRequest(`{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":0}`)
		if response.Code != 200 || response.Status != 0 || response.Data != language.GetValue("txtNoRgbOverride") {
			t.Fatalf("imported get response = %#v", response)
		}
	})

	t.Run("set", func(t *testing.T) {
		seams := installRGBOverrideRequestSeams(t)
		installImportedRGBOverrideTarget(t, seams)
		response := rgbOverrideSetRequest(`{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":0,"enabled":true,"speed":2}`)
		if response.Code != 200 || response.Status != 0 || response.Message != language.GetValue("txtRgbOverrideFailed") {
			t.Fatalf("imported set response = %#v", response)
		}
	})
}

func TestProcessSetRgbOverrideMalformedJSON(t *testing.T) {
	logger.Init()
	seams := installRGBOverrideRequestSeams(t)
	seams.lookup = func(string) (*common.Device, *openrgbimport.Device, bool) {
		t.Fatal("malformed request reached target lookup")
		return nil, nil, false
	}
	response := rgbOverrideSetRequest(`{"deviceId":`)
	if response.Code != 200 || response.Status != 0 || response.Message != language.GetValue("txtUnableToValidateRequest") {
		t.Fatalf("malformed JSON response = %#v", response)
	}
}

func TestProcessSetRgbOverrideNativeFallbackCompatibility(t *testing.T) {
	for _, result := range []struct {
		name       string
		value      uint8
		wantStatus int
	}{
		{name: "success", value: 1, wantStatus: 1},
		{name: "failure", value: 0, wantStatus: 0},
	} {
		t.Run(result.name, func(t *testing.T) {
			seams := installRGBOverrideRequestSeams(t)
			seams.get = func(serial string) interface{} {
				if serial != "native-device-test" {
					t.Fatalf("native lookup serial = %q", serial)
				}
				return struct{}{}
			}
			var calls []rgbOverrideMethodCall
			seams.call = func(serial, method string, args ...interface{}) []reflect.Value {
				if serial != "native-device-test" {
					t.Fatalf("native dispatch serial = %q", serial)
				}
				calls = append(calls, rgbOverrideMethodCall{method: method, args: append([]interface{}(nil), args...)})
				return []reflect.Value{reflect.ValueOf(result.value)}
			}

			response := rgbOverrideSetRequest(`{"deviceId":"native-device-test","channelId":4,"subDeviceId":-9,"enabled":true,"startColor":{"red":-1,"green":300,"blue":2},"middleColor":{"red":3,"green":4,"blue":5},"endColor":{"red":6,"green":7,"blue":8},"speed":2.5}`)
			if response.Code != 200 || response.Status != result.wantStatus {
				t.Fatalf("native response = %#v, want status %d", response, result.wantStatus)
			}
			if len(calls) != 1 || calls[0].method != "ProcessSetRgbOverride" || len(calls[0].args) != 7 {
				t.Fatalf("native calls = %#v", calls)
			}
			if calls[0].args[0] != 4 || calls[0].args[1] != -9 || calls[0].args[2] != true || calls[0].args[6] != 2.5 {
				t.Fatalf("native arguments = %#v", calls[0].args)
			}
			if got := calls[0].args[3].(rgb.Color); got.Red != -1 || got.Green != 300 {
				t.Fatalf("native color validation changed forwarded value %#v", got)
			}
		})
	}

	t.Run("omitted speed retains reflective get", func(t *testing.T) {
		seams := installRGBOverrideRequestSeams(t)
		seams.get = func(string) interface{} { return struct{}{} }
		type nativeOverride struct{ RgbModeSpeed float64 }
		var calls []rgbOverrideMethodCall
		seams.call = func(_ string, method string, args ...interface{}) []reflect.Value {
			calls = append(calls, rgbOverrideMethodCall{method: method, args: append([]interface{}(nil), args...)})
			if method == "ProcessGetRgbOverride" {
				return []reflect.Value{reflect.ValueOf(&nativeOverride{RgbModeSpeed: 4.25})}
			}
			return []reflect.Value{reflect.ValueOf(uint8(1))}
		}

		response := rgbOverrideSetRequest(`{"deviceId":"native-device-test","channelId":2,"subDeviceId":3,"enabled":false}`)
		if response.Status != 1 || len(calls) != 2 || calls[0].method != "ProcessGetRgbOverride" || calls[1].method != "ProcessSetRgbOverride" {
			t.Fatalf("native omitted-speed response/calls = %#v / %#v", response, calls)
		}
		if got := calls[1].args[6]; got != 4.25 {
			t.Fatalf("native preserved speed = %#v, want 4.25", got)
		}
	})
}

func TestProcessGetRgbOverrideNativeFallbackCompatibility(t *testing.T) {
	seams := installRGBOverrideRequestSeams(t)
	seams.get = func(serial string) interface{} {
		if serial != "native-device-test" {
			t.Fatalf("native get lookup serial = %q, want native-device-test", serial)
		}
		return struct{}{}
	}
	want := struct{ Enabled bool }{Enabled: true}
	seams.call = func(serial, method string, args ...interface{}) []reflect.Value {
		if serial != "native-device-test" {
			t.Fatalf("native get dispatch serial = %q, want native-device-test", serial)
		}
		if method != "ProcessGetRgbOverride" || len(args) != 2 || args[0] != 2 || args[1] != 3 {
			t.Fatalf("native get dispatch = %q %#v", method, args)
		}
		return []reflect.Value{reflect.ValueOf(want)}
	}

	response := rgbOverrideGetRequest(`{"deviceId":"native-device-test","channelId":2,"subDeviceId":3}`)
	if response.Code != 200 || response.Status != 1 || !reflect.DeepEqual(response.Data, want) {
		t.Fatalf("native get response = %#v", response)
	}
}
