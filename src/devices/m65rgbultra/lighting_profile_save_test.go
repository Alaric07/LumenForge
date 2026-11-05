package m65rgbultra

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
	// Loaded Ultra assignments retain the source tilt-upgrade behavior. Supply
	// the complete persisted shape so this test measures linkage, not that upgrade.
	for i := 0; len(d.KeyAssignment) < 19; i++ {
		d.KeyAssignment[10000+i] = inputmanager.KeyAssignment{Name: "Fixture"}
	}
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
			if saved != nil && saved.Path != filepath.Join(pwd, "database", "profiles", d.Serial+"-"+name+".json") {
				t.Fatal("named path changed", saved.Path)
			}
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

func TestCanonicalProfileActivationSecondWriteFailureRestoresActiveMarker(t *testing.T) {
	d := profileSaveFixture(t, "X")
	active, inactive := d.DeviceProfile, d.UserProfiles["Z"]
	before, files := inputJSON(t, d), profileSaveFiles(t)
	snapshot, _ := d.LightingSnapshot()
	worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
	writes := 0
	d.lightingProfileWrite = func(path string, data []byte) error {
		writes++
		if writes == 2 {
			return errors.New("target profile persistence failed")
		}
		return writeLightingProfile(path, data)
	}
	if d.ChangeDeviceProfile("Z") != 0 || writes != 3 {
		t.Fatal("failure hidden or no restoration", writes)
	}
	after, _ := d.LightingSnapshot()
	if !reflect.DeepEqual(files, profileSaveFiles(t)) || !bytes.Equal(before, inputJSON(t, d)) || !reflect.DeepEqual(snapshot, after) || d.DeviceProfile != active || d.UserProfiles["X"] != active || d.UserProfiles["Z"] != inactive || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
		t.Fatal("failed activation published or left wrong marker")
	}
}

func TestUltraProfileActivationAssignmentUpgradeFailurePublishesNothing(t *testing.T) {
	d := profileSaveFixture(t, "X")
	// Source upgrades a short assignment map. Failure must occur before either
	// profile marker changes or the target bindings become active.
	path := filepath.Join(pwd, "database", "key-assignments", d.UserProfiles["Z"].KeyAssignmentHash+".json")
	requireLighting(t, os.WriteFile(path, []byte(`{"1":{"Default":true}}`), 0600))
	before, files := inputJSON(t, d), profileSaveFiles(t)
	worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
	oldAssignments := d.inputAssignmentsSnapshot()
	profileWrites := 0
	d.lightingAssignmentWrite = func(string, []byte) error { return errors.New("assignment upgrade failure") }
	d.lightingProfileWrite = func(string, []byte) error { profileWrites++; return nil }
	if d.ChangeDeviceProfile("Z") != 0 || profileWrites != 0 || !reflect.DeepEqual(files, profileSaveFiles(t)) || !bytes.Equal(before, inputJSON(t, d)) || !reflect.DeepEqual(oldAssignments, d.inputAssignmentsSnapshot()) || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
		t.Fatal("failed upgrade published profile/bindings/runtime")
	}
}

func inactiveOverwriteFixture(t *testing.T, name string) (*Device, *DeviceProfile, string) {
	t.Helper()
	d := profileSaveFixture(t, "X")
	existing := cloneLightingProfile(*d.UserProfiles["Z"])
	existing.Label = "prior inactive snapshot"
	if name == "default" {
		existing.Path = filepath.Join(pwd, "database", "profiles", d.Serial+".json")
	} else {
		requireLighting(t, os.Remove(existing.Path))
		// A stored path need not equal the synthesized <serial>-<name>.json.
		// The loader still identifies this source-compatible filename as logical Z.
		existing.Path = filepath.Join(pwd, "database", "profiles", d.Serial+"-Z-preserved.json")
	}
	d.UserProfiles[name] = &existing
	requireLighting(t, d.persistLightingProfile(existing))
	d.rebuildProfileOrder()
	alternate := filepath.Join(pwd, "database", "profiles", d.Serial+"-"+name+".json")
	return d, &existing, alternate
}

func TestCanonicalInactiveOverwritePreservesExistingPathAndReload(t *testing.T) {
	for _, name := range []string{"default", "Z"} {
		t.Run(name, func(t *testing.T) {
			d, existing, alternate := inactiveOverwriteFixture(t, name)
			before, files := inputJSON(t, d), profileSaveFiles(t)
			active := d.DeviceProfile
			worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
			snapshot, _ := d.LightingSnapshot()
			calls := 0
			d.lightingProfileWrite = func(path string, data []byte) error {
				calls++
				if path != existing.Path || d.UserProfiles[name] != existing {
					t.Fatal("wrong path or early publication", path)
				}
				var candidate DeviceProfile
				requireLighting(t, json.Unmarshal(data, &candidate))
				assignments, err := os.ReadFile(filepath.Join(pwd, "database", "key-assignments", candidate.KeyAssignmentHash+".json"))
				requireLighting(t, err)
				var keys map[int]inputmanager.KeyAssignment
				requireLighting(t, json.Unmarshal(assignments, &keys))
				if !reflect.DeepEqual(keys, d.KeyAssignment) {
					t.Fatal("assignment linkage/order")
				}
				return writeLightingProfile(path, data)
			}
			if d.SaveUserProfile(name) != 1 || calls != 1 {
				t.Fatal("overwrite failed")
			}
			saved := d.UserProfiles[name]
			if saved == nil || saved == existing || saved == active || saved.Active || saved.Path != existing.Path || saved.Label != active.Label || d.DeviceProfile != active || !bytes.Equal(before, inputJSON(t, d)) {
				t.Fatal("inactive overwrite publication")
			}
			if _, err := os.Stat(alternate); !os.IsNotExist(err) {
				t.Fatal("alternate profile created", alternate, err)
			}
			data, err := os.ReadFile(existing.Path)
			requireLighting(t, err)
			relative, _ := filepath.Rel(filepath.Join(pwd, "database"), existing.Path)
			if string(data) == files[filepath.ToSlash(relative)] {
				t.Fatal("original profile was not updated")
			}
			var stored DeviceProfile
			requireLighting(t, json.Unmarshal(data, &stored))
			if !reflect.DeepEqual(stored, *saved) || stored.KeyAssignmentHash == existing.KeyAssignmentHash {
				t.Fatal("stored snapshot/hash")
			}
			after, _ := d.LightingSnapshot()
			if !reflect.DeepEqual(snapshot, after) || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
				t.Fatal("save changed Lighting/runtime")
			}
			reloaded := &Device{Serial: d.Serial}
			reloaded.loadDeviceProfiles()
			if len(reloaded.UserProfiles) != len(d.UserProfiles) || reloaded.DeviceProfile != reloaded.UserProfiles["X"] || !reflect.DeepEqual(*reloaded.UserProfiles[name], *saved) {
				t.Fatal("reload lost logical profile or active identity")
			}
			entries, err := os.ReadDir(filepath.Join(pwd, "database", "profiles"))
			requireLighting(t, err)
			if len(entries) != len(d.UserProfiles) {
				t.Fatal("duplicate physical profile", entries)
			}
			if name == "default" {
				count := 0
				for _, n := range reloaded.ProfileOrder {
					if n == "default" {
						count++
					}
				}
				if count != 1 {
					t.Fatal("duplicate logical default", reloaded.ProfileOrder)
				}
			}
			// The saved hash also selects the correct bindings after restart.
			reloaded.DeviceProfile = reloaded.UserProfiles[name]
			reloaded.loadKeyAssignments()
			if !reflect.DeepEqual(reloaded.KeyAssignment, d.KeyAssignment) {
				t.Fatal("reloaded assignment linkage")
			}
		})
	}
}

func TestCanonicalInactiveExistingPathFailurePublishesNothing(t *testing.T) {
	for _, name := range []string{"default", "Z"} {
		for _, failure := range []string{"assignment", "profile"} {
			t.Run(name+"/"+failure, func(t *testing.T) {
				d, existing, alternate := inactiveOverwriteFixture(t, name)
				before, files := inputJSON(t, d), profileSaveFiles(t)
				active := d.DeviceProfile
				order := append([]string(nil), d.ProfileOrder...)
				snapshot, _ := d.LightingSnapshot()
				worker, native, external := d.lightingWorker, d.nativeGeneration, d.externalGeneration
				calls := 0
				d.lightingProfileWrite = func(path string, _ []byte) error {
					calls++
					if path != existing.Path {
						t.Fatal("failure attempted alternate path", path)
					}
					return errors.New("profile failure")
				}
				if failure == "assignment" {
					d.lightingAssignmentWrite = func(string, []byte) error { return errors.New("assignment failure") }
				}
				if d.SaveUserProfile(name) != 0 {
					t.Fatal("failure reported success")
				}
				expected := 1
				if failure == "assignment" {
					expected = 0
				}
				if calls != expected {
					t.Fatal("persistence order", calls)
				}
				after, _ := d.LightingSnapshot()
				if !reflect.DeepEqual(files, profileSaveFiles(t)) || d.UserProfiles[name] != existing || d.DeviceProfile != active || !bytes.Equal(before, inputJSON(t, d)) || !reflect.DeepEqual(order, d.ProfileOrder) || !reflect.DeepEqual(snapshot, after) || d.lightingWorker != worker || d.nativeGeneration != native || d.externalGeneration != external {
					t.Fatal("failure published or left artifacts")
				}
				if _, err := os.Stat(alternate); !os.IsNotExist(err) {
					t.Fatal("alternate file after failure", err)
				}
			})
		}
	}
}
