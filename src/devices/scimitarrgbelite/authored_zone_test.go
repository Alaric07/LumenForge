package scimitarrgbelite

import (
	"testing"

	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

type authoredZoneLightingSource struct{}
func (authoredZoneLightingSource) resolve() (scimitarEliteResolvedLighting, error) { return scimitarEliteResolvedLighting{}, nil }
func (authoredZoneLightingSource) resolveEffectSettings(string) (lightingsettings.EffectSettings, error) { return lightingsettings.EffectSettings{}, nil }
func (authoredZoneLightingSource) resolveEffectSettingsWithStatus(string) (lightingsettings.Resolution, error) { return lightingsettings.Resolution{}, nil }
func (authoredZoneLightingSource) selectedEffect() (string, error) { return "mouse", nil }
func (authoredZoneLightingSource) setSelectedEffect(string) error { return nil }
func (authoredZoneLightingSource) setEffectSettings(string, lightingsettings.EffectSettings) error { return nil }
func (authoredZoneLightingSource) deleteEffectSettings(string) (bool, error) { return false, nil }
func (authoredZoneLightingSource) brightness() (uint8, error) { return 100, nil }
func (authoredZoneLightingSource) setBrightness(uint8) error { return nil }

func TestScimitarEliteMouseSnapshotPresentsOnlyAuthoredZones(t *testing.T) {
	profile := &DeviceProfile{DPIColor: &rgb.Color{Red: 99}, ZoneColors: map[int]ZoneColors{
		4: {Name: "Logo", Color: &rgb.Color{Red: 4}, ColorIndex: []int{4}, LEDIndex: 4, LEDIndexPosition: 4},
		1: {Name: "Front", Color: &rgb.Color{Red: 1}, ColorIndex: []int{1}, LEDIndex: 1, LEDIndexPosition: 1},
		3: {Name: "Side", Color: &rgb.Color{Red: 3}, ColorIndex: []int{3}, LEDIndex: 3, LEDIndexPosition: 3},
		2: {Name: "Scroll", Color: &rgb.Color{Red: 2}, ColorIndex: []int{2}, LEDIndex: 2, LEDIndexPosition: 2},
	}}
	device := &Device{Serial: "elite-authored", DeviceProfile: profile, lightingSource: authoredZoneLightingSource{}}
	snapshot, ok := device.LightingSnapshot()
	if !ok || snapshot.AuthoredZoneEditor == nil || snapshot.AuthoredZoneEditor.HasGroups || len(snapshot.AuthoredZoneEditor.Zones) != 4 { t.Fatalf("snapshot = %#v", snapshot) }
	for index, label := range []string{"Front", "Scroll", "Side", "Logo"} {
		zone := snapshot.AuthoredZoneEditor.Zones[index]
		if zone.Label != label || zone.GroupID != "" || zone.HasGeometry { t.Fatalf("zone %d = %#v", index, zone) }
	}
	snapshot.AuthoredZoneEditor.Zones[0].ColorHex = "#ffffff"
	if profile.ZoneColors[1].Color.Red != 1 { t.Fatal("snapshot aliases ZoneColors") }
}

func TestScimitarEliteAuthoredMutationRejectsUnsupportedScopes(t *testing.T) {
	device := &Device{DeviceProfile: &DeviceProfile{ZoneColors: map[int]ZoneColors{1: {Name: "Front", Color: &rgb.Color{}}}}}
	if err := device.SetLightingZoneColor("mouse", "group", "", "1", rgb.Color{}); err == nil { t.Fatal("group mutation succeeded") }
	if err := device.SetLightingZoneColor("static", "all", "", "", rgb.Color{}); err == nil { t.Fatal("shared effect mutation succeeded") }
	if err := device.SetLightingZoneColor("mouse", "zone", "99", "", rgb.Color{}); err == nil { t.Fatal("unknown zone mutation succeeded") }
}
