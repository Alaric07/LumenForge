package m65rgbelite

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func TestCanonicalProfileDeletionRebuildsRotationOrder(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.DeviceProfile.RgbOff = true
	d.userRGBOff, d.schedulerDark = true, true
	d.UserProfiles = make(map[string]*DeviceProfile)
	original := make(map[string]DeviceProfile)
	diskBefore := make(map[string][]byte)
	// Deliberately insert out of order; rebuilding must sort the surviving names.
	for _, name := range []string{"c", "a", "b"} {
		p := cloneLightingProfile(*d.DeviceProfile)
		p.Path = filepath.Join(filepath.Dir(p.Path), name+".json")
		p.Label, p.Active = name, name == "a"
		data, err := json.MarshalIndent(p, "", "    ")
		requireLighting(t, err)
		requireLighting(t, os.WriteFile(p.Path, data, 0600))
		d.UserProfiles[name] = &p
		original[name] = cloneLightingProfile(p)
		diskBefore[name] = data
	}
	d.DeviceProfile = d.UserProfiles["a"]
	d.rebuildProfileOrder()
	if !reflect.DeepEqual(d.ProfileOrder, []string{"a", "b", "c"}) {
		t.Fatal(d.ProfileOrder)
	}
	saves := 0
	d.lightingProfileWrite = func(path string, data []byte) error {
		saves++
		return os.WriteFile(path, data, 0600)
	}
	worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
	d.lightingWorker = worker
	beforeLighting, _ := d.LightingSnapshot()
	beforeInput := inputJSON(t, d)
	native, external := d.nativeGeneration, d.externalGeneration
	// Active-profile deletion keeps its original return code and file/map/order.
	if d.DeleteDeviceProfile("a") != 2 || len(d.UserProfiles) != 3 || !reflect.DeepEqual(d.ProfileOrder, []string{"a", "b", "c"}) {
		t.Fatal("active deletion semantics changed")
	}
	if d.DeleteDeviceProfile("b") != 1 {
		t.Fatal("delete")
	}
	if _, found := d.UserProfiles["b"]; found {
		t.Fatal("deleted profile retained")
	}
	if !reflect.DeepEqual(d.ProfileOrder, []string{"a", "c"}) {
		t.Fatal("stale/unsorted order", d.ProfileOrder)
	}
	if _, err := os.Stat(original["b"].Path); !os.IsNotExist(err) {
		t.Fatal("deleted file remains", err)
	}
	for _, name := range []string{"a", "c"} {
		data, err := os.ReadFile(original[name].Path)
		requireLighting(t, err)
		if !bytes.Equal(data, diskBefore[name]) {
			t.Fatal("deletion rewrote survivor", name)
		}
	}
	afterLighting, _ := d.LightingSnapshot()
	if saves != 0 || !bytes.Equal(beforeInput, inputJSON(t, d)) || !reflect.DeepEqual(beforeLighting, afterLighting) || !d.userRGBOff || !d.schedulerDark || d.lightingWorker != worker || native != d.nativeGeneration || external != d.externalGeneration {
		t.Fatal("deletion disturbed profile, overrides, Lighting or worker/session")
	}
	select {
	case <-worker.stop:
		t.Fatal("deletion cancelled worker")
	default:
	}
	// Rotation retains its normal profile lifecycle; only the stale name is gone.
	// Each press must advance, including repeated wrap-around to the first entry.
	for _, want := range []string{"c", "a", "c", "a", "c", "a"} {
		d.rotateCanonicalProfile()
		if d.DeviceProfile != d.UserProfiles[want] || d.DeviceProfile.Label != want {
			t.Fatal("rotation stalled", want, d.DeviceProfile.Label)
		}
		if _, found := d.UserProfiles["b"]; found {
			t.Fatal("deleted profile reappeared")
		}
		snapshot, usable := d.LightingSnapshot()
		if !usable || !reflect.DeepEqual(beforeLighting, snapshot) || !d.userRGBOff || !d.schedulerDark {
			t.Fatal("rotation changed canonical Lighting/overrides")
		}
		for _, name := range []string{"a", "c"} {
			data, err := os.ReadFile(original[name].Path)
			requireLighting(t, err)
			var stored DeviceProfile
			requireLighting(t, json.Unmarshal(data, &stored))
			expected := original[name]
			expected.Active = name == want
			if !reflect.DeepEqual(expected, stored) {
				t.Fatal("rotation changed unrelated profile persistence", name)
			}
		}
	}
}
