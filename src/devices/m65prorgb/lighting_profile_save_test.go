package m65prorgb

import (
	"LumenForge/src/inputmanager"
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"testing"
)

func profileSaveFixture(t *testing.T, activeName string) *Device {
	t.Helper()
	d, rt := lightingFixture(t, false)
	root := t.TempDir()
	previous := pwd
	pwd = root
	t.Cleanup(func() { pwd = previous })
	for _, dir := range []string{"profiles", "key-assignments"} {
		requireLighting(t, os.MkdirAll(filepath.Join(root, "database", dir), 0700))
	}
	// Use the existing loader's serial/name filename contract for real reloads.
	d.Serial = "m65test"
	p := cloneLightingProfile(*d.DeviceProfile)
	p.Serial, p.Label, p.KeyAssignmentHash, p.RgbOff = d.Serial, activeName, "oldassign", true
	p.Path = filepath.Join(root, "database", "profiles", d.Serial+"-"+activeName+".json")
	if activeName == "default" {
		p.Path = filepath.Join(root, "database", "profiles", d.Serial+".json")
	}
	d.DeviceProfile = &p
	d.UserProfiles = map[string]*DeviceProfile{activeName: d.DeviceProfile}
	q := cloneLightingProfile(p)
	q.Active, q.Label, q.KeyAssignmentHash = false, "Z", "inactiveassign"
	q.Path = filepath.Join(root, "database", "profiles", d.Serial+"-Z.json")
	d.UserProfiles["Z"] = &q
	d.rebuildProfileOrder()
	d.KeyAssignment = map[int]inputmanager.KeyAssignment{1: {Name: "Left Button", Default: true}}
	for _, profile := range d.UserProfiles {
		requireLighting(t, d.persistLightingProfile(*profile))
	}
	for _, hash := range []string{"oldassign", "inactiveassign"} {
		requireLighting(t, os.WriteFile(filepath.Join(root, "database", "key-assignments", hash+".json"), []byte("{}"), 0600))
	}
	requireLighting(t, d.attachLightingSource(rt))
	d.schedulerDark = true
	d.lightingWorker = &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
	return d
}

func profileSaveFiles(t *testing.T) map[string]string {
	t.Helper()
	files := make(map[string]string)
	for _, dir := range []string{"profiles", "key-assignments"} {
		base := filepath.Join(pwd, "database", dir)
		entries, err := os.ReadDir(base)
		requireLighting(t, err)
		for _, entry := range entries {
			data, err := os.ReadFile(filepath.Join(base, entry.Name()))
			requireLighting(t, err)
			files[dir+"/"+entry.Name()] = string(data)
		}
	}
	return files
}

func TestCanonicalActiveProfileSavePreservesIdentityAndRestart(t *testing.T) {
	for _, name := range []string{"X", "default"} {
		t.Run(name, func(t *testing.T) {
			d := profileSaveFixture(t, name)
			active, path, oldHash := d.DeviceProfile, d.DeviceProfile.Path, d.DeviceProfile.KeyAssignmentHash
			snapshot, _ := d.LightingSnapshot()
			worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
			calls := 0
			d.lightingProfileWrite = func(target string, data []byte) error {
				calls++
				// Assignment file must exist before committing the profile link, while
				// no candidate fields/pointers may yet have been published.
				var proposal DeviceProfile
				requireLighting(t, json.Unmarshal(data, &proposal))
				if d.DeviceProfile != active || d.UserProfiles[name] != active || d.DeviceProfile.KeyAssignmentHash != oldHash {
					t.Fatal("published before profile persistence")
				}
				assignments, err := os.ReadFile(filepath.Join(pwd, "database", "key-assignments", proposal.KeyAssignmentHash+".json"))
				requireLighting(t, err)
				var keys map[int]inputmanager.KeyAssignment
				requireLighting(t, json.Unmarshal(assignments, &keys))
				if !reflect.DeepEqual(keys, d.KeyAssignment) {
					t.Fatal("assignment linkage/order")
				}
				return writeLightingProfile(target, data)
			}
			if d.saveCanonicalUserProfile(name) != 1 || calls != 1 {
				t.Fatal("active save failed")
			}
			if d.DeviceProfile != active || d.UserProfiles[name] != active || !active.Active || active.Path != path || active.KeyAssignmentHash == oldHash {
				t.Fatal("active identity, marker or path changed")
			}
			data, err := os.ReadFile(path)
			requireLighting(t, err)
			var stored DeviceProfile
			requireLighting(t, json.Unmarshal(data, &stored))
			if !stored.Active || !reflect.DeepEqual(stored, *active) {
				t.Fatal("persisted active profile mismatch")
			}
			wantOrder := []string{"X", "Z"}
			if name == "default" {
				wantOrder = []string{"Z", "default"}
			}
			if !reflect.DeepEqual(d.ProfileOrder, wantOrder) {
				t.Fatal(d.ProfileOrder)
			}
			after, _ := d.LightingSnapshot()
			if !reflect.DeepEqual(snapshot, after) || !d.userRGBOff || !d.schedulerDark || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
				t.Fatal("save disturbed Lighting/override/worker/session")
			}
			select {
			case <-worker.stop:
				t.Fatal("save cancelled worker")
			default:
			}
			// Read the actual persisted files through the production reload selector.
			reloaded := &Device{Serial: d.Serial}
			reloaded.loadDeviceProfiles()
			if reloaded.DeviceProfile == nil || reloaded.DeviceProfile != reloaded.UserProfiles[name] || reloaded.DeviceProfile.KeyAssignmentHash != active.KeyAssignmentHash || !reloaded.DeviceProfile.Active {
				t.Fatal("reload lost active profile")
			}
			reloaded.loadKeyAssignments()
			if !reflect.DeepEqual(reloaded.KeyAssignment, d.KeyAssignment) {
				t.Fatal("reload lost saved assignment linkage")
			}
			d.lightingProfileWrite = nil
			// Pointer-based rotation must locate the saved active object, then wrap.
			d.rotateCanonicalProfile()
			if d.DeviceProfile != d.UserProfiles["Z"] {
				t.Fatal("rotation failed to identify saved active profile")
			}
			d.rotateCanonicalProfile()
			if d.DeviceProfile != d.UserProfiles[name] {
				t.Fatal("rotation failed to wrap")
			}
		})
	}
}

func TestCanonicalProfileSaveNewAndInactiveNamesRemainInactive(t *testing.T) {
	for _, name := range []string{"Y", "Z"} {
		t.Run(name, func(t *testing.T) {
			d := profileSaveFixture(t, "X")
			snapshot, _ := d.LightingSnapshot()
			active := d.DeviceProfile
			before := inputJSON(t, d)
			activeDisk, err := os.ReadFile(active.Path)
			requireLighting(t, err)
			worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
			if d.SaveUserProfile(name) != 1 {
				t.Fatal("new/inactive save failed")
			}
			saved := d.UserProfiles[name]
			if saved == nil || saved == active || saved.Active || d.DeviceProfile != active || d.UserProfiles["X"] != active || !bytes.Equal(before, inputJSON(t, d)) {
				t.Fatal("inactive snapshot changed active profile")
			}
			wantOrder := []string{"X", "Y", "Z"}
			if name == "Z" {
				wantOrder = []string{"X", "Z"}
			}
			if !reflect.DeepEqual(d.ProfileOrder, wantOrder) {
				t.Fatal(d.ProfileOrder)
			}
			data, err := os.ReadFile(active.Path)
			requireLighting(t, err)
			if !bytes.Equal(data, activeDisk) {
				t.Fatal("active file overwritten")
			}
			data, err = os.ReadFile(saved.Path)
			requireLighting(t, err)
			var stored DeviceProfile
			requireLighting(t, json.Unmarshal(data, &stored))
			if stored.Active || stored.KeyAssignmentHash == "oldassign" || stored.KeyAssignmentHash == "inactiveassign" || !reflect.DeepEqual(stored, *saved) {
				t.Fatal("saved snapshot/linkage")
			}
			if _, err = os.Stat(filepath.Join(pwd, "database", "key-assignments", stored.KeyAssignmentHash+".json")); err != nil {
				t.Fatal(err)
			}
			after, _ := d.LightingSnapshot()
			if !reflect.DeepEqual(snapshot, after) || !d.userRGBOff || !d.schedulerDark || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
				t.Fatal("save disturbed runtime")
			}
		})
	}
}

func TestCanonicalProfileSaveFailurePublishesNothing(t *testing.T) {
	for _, name := range []string{"X", "Y", "Z"} {
		t.Run(name, func(t *testing.T) {
			d := profileSaveFixture(t, "X")
			active, inactive := d.DeviceProfile, d.UserProfiles["Z"]
			before, files := inputJSON(t, d), profileSaveFiles(t)
			order := append([]string(nil), d.ProfileOrder...)
			worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
			calls := 0
			d.lightingProfileWrite = func(_ string, data []byte) error {
				calls++
				var proposal DeviceProfile
				requireLighting(t, json.Unmarshal(data, &proposal))
				if _, err := os.Stat(filepath.Join(pwd, "database", "key-assignments", proposal.KeyAssignmentHash+".json")); err != nil {
					t.Fatal("assignment was not saved first", err)
				}
				return errors.New("profile save failure")
			}
			if d.saveCanonicalUserProfile(name) != 0 || calls != 1 {
				t.Fatal("failed save reported success")
			}
			if !reflect.DeepEqual(files, profileSaveFiles(t)) || !bytes.Equal(before, inputJSON(t, d)) || d.DeviceProfile != active || d.UserProfiles["X"] != active || d.UserProfiles["Z"] != inactive || len(d.UserProfiles) != 2 || !reflect.DeepEqual(order, d.ProfileOrder) {
				t.Fatal("failed save published state or orphaned assignment")
			}
			if !active.Active || !d.userRGBOff || !d.schedulerDark || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
				t.Fatal("failed save disturbed runtime")
			}
		})
	}
}

func TestCanonicalActiveProfileAssignmentFailurePublishesNothing(t *testing.T) {
	d := profileSaveFixture(t, "X")
	active := d.DeviceProfile
	before, files := inputJSON(t, d), profileSaveFiles(t)
	order := append([]string(nil), d.ProfileOrder...)
	worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
	profileWrites := 0
	d.lightingProfileWrite = func(string, []byte) error { profileWrites++; return nil }
	// Make assignment creation fail deterministically without changing the old
	// assignment contents or relying on filesystem permission/user assumptions.
	directory := filepath.Join(pwd, "database", "key-assignments")
	backup := directory + "-saved"
	requireLighting(t, os.Rename(directory, backup))
	requireLighting(t, os.WriteFile(directory, []byte("not a directory"), 0600))
	result := d.saveCanonicalUserProfile("X")
	requireLighting(t, os.Remove(directory))
	requireLighting(t, os.Rename(backup, directory))
	if result != 0 || profileWrites != 0 || !reflect.DeepEqual(files, profileSaveFiles(t)) || !bytes.Equal(before, inputJSON(t, d)) || d.DeviceProfile != active || d.UserProfiles["X"] != active || !active.Active || !reflect.DeepEqual(order, d.ProfileOrder) {
		t.Fatal("assignment failure changed profile/linkage")
	}
	if !d.userRGBOff || !d.schedulerDark || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
		t.Fatal("assignment failure changed runtime")
	}
}
