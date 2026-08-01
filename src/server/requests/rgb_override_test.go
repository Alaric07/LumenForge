package requests

import (
	"LumenForge/src/common"
	"LumenForge/src/devices/openrgbimport"
	"LumenForge/src/language"
	"LumenForge/src/logger"
	"LumenForge/src/rgb"
	"bytes"
	"errors"
	"net/http/httptest"
	"reflect"
	"strings"
	"testing"
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
	set    func(*openrgbimport.Device, string, int, int, bool, rgb.Color, rgb.Color, rgb.Color, *float64) error
	log    func(error)
}

func installRGBOverrideRequestSeams(t *testing.T) *rgbOverrideRequestSeams {
	t.Helper()
	previousLookup := lookupOpenRGBOverrideDevice
	previousGet := getRGBOverrideDevice
	previousCall := callRGBOverrideDeviceMethod
	previousSet := setOpenRGBOverride
	previousLog := logOpenRGBOverrideFailure
	seams := &rgbOverrideRequestSeams{
		lookup: func(string) (*common.Device, *openrgbimport.Device, bool) { return nil, nil, false },
		get:    func(string) interface{} { return nil },
		call:   func(string, string, ...interface{}) []reflect.Value { return nil },
		set: func(*openrgbimport.Device, string, int, int, bool, rgb.Color, rgb.Color, rgb.Color, *float64) error {
			return nil
		},
		log: func(error) {},
	}
	lookupOpenRGBOverrideDevice = func(serial string) (*common.Device, *openrgbimport.Device, bool) {
		return seams.lookup(serial)
	}
	getRGBOverrideDevice = func(serial string) interface{} { return seams.get(serial) }
	callRGBOverrideDeviceMethod = func(serial, method string, args ...interface{}) []reflect.Value {
		return seams.call(serial, method, args...)
	}
	setOpenRGBOverride = func(device *openrgbimport.Device, serial string, channelId, subDeviceId int, enabled bool, startColor, endColor, middleColor rgb.Color, speed *float64) error {
		return seams.set(device, serial, channelId, subDeviceId, enabled, startColor, endColor, middleColor, speed)
	}
	logOpenRGBOverrideFailure = func(err error) { seams.log(err) }
	t.Cleanup(func() {
		lookupOpenRGBOverrideDevice = previousLookup
		getRGBOverrideDevice = previousGet
		callRGBOverrideDeviceMethod = previousCall
		setOpenRGBOverride = previousSet
		logOpenRGBOverrideFailure = previousLog
	})
	return seams
}

func rgbOverrideRequest(body string) *Payload {
	request := httptest.NewRequest("POST", "/api/color/setOverride", bytes.NewBufferString(body))
	return ProcessSetRgbOverride(request)
}

func validOpenRGBOverrideRequestTarget(seams *rgbOverrideRequestSeams) (*common.Device, *openrgbimport.Device) {
	device := &openrgbimport.Device{Serial: rgbOverrideRequestSerial, IsOpenRGB: true}
	wrapper := &common.Device{Serial: rgbOverrideRequestSerial, Instance: device}
	seams.lookup = func(string) (*common.Device, *openrgbimport.Device, bool) {
		return wrapper, device, true
	}
	return wrapper, device
}

func TestProcessSetRgbOverrideOpenRGBDispatch(t *testing.T) {
	t.Run("valid controller-wide request preserves omitted speed", func(t *testing.T) {
		seams := installRGBOverrideRequestSeams(t)
		_, device := validOpenRGBOverrideRequestTarget(seams)
		called := false
		seams.set = func(gotDevice *openrgbimport.Device, serial string, channelId, subDeviceId int, enabled bool, startColor, endColor, middleColor rgb.Color, speed *float64) error {
			called = true
			if gotDevice != device || serial != rgbOverrideRequestSerial || channelId != 0 || subDeviceId != 0 || !enabled {
				t.Fatalf("OpenRGB dispatch target = %#v, %q, %d/%d, enabled %t", gotDevice, serial, channelId, subDeviceId, enabled)
			}
			if speed != nil {
				t.Fatalf("omitted speed forwarded as %v, want nil for importer-side preservation", *speed)
			}
			if startColor.Red != 0 || startColor.Green != 1 || startColor.Blue != 2 ||
				middleColor.Red != 3 || endColor.Red != 6 {
				t.Fatalf("OpenRGB colors forwarded incorrectly: start %#v, middle %#v, end %#v", startColor, middleColor, endColor)
			}
			return nil
		}
		seams.get = func(string) interface{} {
			t.Fatal("OpenRGB request fell through to native lookup")
			return nil
		}
		seams.call = func(string, string, ...interface{}) []reflect.Value {
			t.Fatal("OpenRGB request fell through to reflective dispatch")
			return nil
		}

		response := rgbOverrideRequest(`{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":0,"enabled":true,"startColor":{"red":0,"green":1,"blue":2,"temperature":20},"middleColor":{"red":3,"green":4,"blue":5,"temperature":40},"endColor":{"red":6,"green":7,"blue":8,"temperature":60}}`)
		if !called || response.Code != 200 || response.Status != 1 || response.Message != language.GetValue("txtRgbOverrideUpdated") {
			t.Fatalf("OpenRGB success response = %#v, called %t", response, called)
		}
	})

	for _, test := range []struct {
		name    string
		prepare func(*common.Device, *openrgbimport.Device, *rgbOverrideRequestSeams)
		body    string
	}{
		{
			name: "nonzero channel",
			body: `{"deviceId":"openrgb-override-request-test","channelId":1,"subDeviceId":0,"speed":2}`,
		},
		{
			name: "nonzero subdevice",
			body: `{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":1,"speed":2}`,
		},
		{
			name: "hidden wrapper",
			body: `{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":0,"speed":2}`,
			prepare: func(wrapper *common.Device, _ *openrgbimport.Device, _ *rgbOverrideRequestSeams) {
				wrapper.Hidden = true
			},
		},
		{
			name: "unavailable wrapper",
			body: `{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":0,"speed":2}`,
			prepare: func(wrapper *common.Device, _ *openrgbimport.Device, _ *rgbOverrideRequestSeams) {
				wrapper.Unavailable = true
			},
		},
		{
			name: "mismatched wrapper instance",
			body: `{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":0,"speed":2}`,
			prepare: func(wrapper *common.Device, _ *openrgbimport.Device, _ *rgbOverrideRequestSeams) {
				wrapper.Instance = &openrgbimport.Device{Serial: rgbOverrideRequestSerial, IsOpenRGB: true}
			},
		},
		{
			name: "mismatched importer identity",
			body: `{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":0,"speed":2}`,
			prepare: func(_ *common.Device, device *openrgbimport.Device, _ *rgbOverrideRequestSeams) {
				device.Serial = "openrgb-other-request-test"
			},
		},
		{
			name: "checked inactive importer failure",
			body: `{"deviceId":"openrgb-override-request-test","channelId":0,"subDeviceId":0,"speed":2}`,
			prepare: func(_ *common.Device, _ *openrgbimport.Device, seams *rgbOverrideRequestSeams) {
				wantErr := errors.New("detached importer at /private/test/path")
				seams.set = func(*openrgbimport.Device, string, int, int, bool, rgb.Color, rgb.Color, rgb.Color, *float64) error {
					return wantErr
				}
				seams.log = func(err error) {
					if err != wantErr {
						t.Fatalf("logged OpenRGB override error = %v, want %v", err, wantErr)
					}
				}
			},
		},
	} {
		t.Run(test.name, func(t *testing.T) {
			seams := installRGBOverrideRequestSeams(t)
			wrapper, device := validOpenRGBOverrideRequestTarget(seams)
			if test.prepare != nil {
				test.prepare(wrapper, device, seams)
			}
			seams.get = func(string) interface{} {
				t.Fatal("rejected OpenRGB target fell through to native lookup")
				return nil
			}
			seams.call = func(string, string, ...interface{}) []reflect.Value {
				t.Fatal("rejected OpenRGB target fell through to reflection")
				return nil
			}

			response := rgbOverrideRequest(test.body)
			if response.Code != 200 || response.Status != 0 {
				t.Fatalf("rejected OpenRGB response = %#v", response)
			}
			if strings.Contains(response.Message, "private") || strings.Contains(response.Message, "detached") || strings.Contains(response.Message, rgbOverrideRequestSerial) {
				t.Fatalf("internal detail leaked in response %q", response.Message)
			}
		})
	}
}

func TestProcessSetRgbOverrideMalformedJSON(t *testing.T) {
	logger.Init()
	seams := installRGBOverrideRequestSeams(t)
	seams.lookup = func(string) (*common.Device, *openrgbimport.Device, bool) {
		t.Fatal("malformed request reached target lookup")
		return nil, nil, false
	}
	response := rgbOverrideRequest(`{"deviceId":`)
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
			seams.set = func(*openrgbimport.Device, string, int, int, bool, rgb.Color, rgb.Color, rgb.Color, *float64) error {
				t.Fatal("native request entered OpenRGB setter")
				return nil
			}

			response := rgbOverrideRequest(`{"deviceId":"native-device-test","channelId":4,"subDeviceId":-9,"enabled":true,"startColor":{"red":-1,"green":300,"blue":2},"middleColor":{"red":3,"green":4,"blue":5},"endColor":{"red":6,"green":7,"blue":8},"speed":2.5}`)
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

		response := rgbOverrideRequest(`{"deviceId":"native-device-test","channelId":2,"subDeviceId":3,"enabled":false}`)
		if response.Status != 1 || len(calls) != 2 || calls[0].method != "ProcessGetRgbOverride" || calls[1].method != "ProcessSetRgbOverride" {
			t.Fatalf("native omitted-speed response/calls = %#v / %#v", response, calls)
		}
		if got := calls[1].args[6]; got != 4.25 {
			t.Fatalf("native preserved speed = %#v, want 4.25", got)
		}
	})
}
