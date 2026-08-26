package lightingpresentation

import "testing"

func TestAuthoredZonePresentationUsesValueCopies(t *testing.T) {
	zone := AuthoredZone{
		ID: "zone-1", Label: "Zone 1", ColorHex: "#123456",
		GroupID: "row-1", GroupLabel: "Row 1",
		HasGeometry: true, Left: 1, Top: 2, Width: 3, Height: 4,
	}
	snapshot := Snapshot{AuthoredZoneEditor: &AuthoredZoneEditor{EffectID: "authored", HasGroups: true, Zones: []AuthoredZone{zone}}}
	copy := Snapshot{AuthoredZoneEditor: &AuthoredZoneEditor{EffectID: snapshot.AuthoredZoneEditor.EffectID, HasGroups: snapshot.AuthoredZoneEditor.HasGroups, Zones: append([]AuthoredZone(nil), snapshot.AuthoredZoneEditor.Zones...)}}
	copy.AuthoredZoneEditor.Zones[0].ColorHex = "#ffffff"
	if snapshot.AuthoredZoneEditor.Zones[0] != zone {
		t.Fatalf("presentation copy aliases source: %#v", snapshot.AuthoredZoneEditor.Zones[0])
	}
}
