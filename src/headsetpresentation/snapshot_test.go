package headsetpresentation

import "testing"

func validEqualizerSnapshot() Snapshot {
	snapshot := Snapshot{Equalizer: make([]EqualizerBand, 0, 10)}
	for id := 1; id <= 10; id++ {
		snapshot.Equalizer = append(snapshot.Equalizer, EqualizerBand{ID: id, Label: "band", Value: 0})
	}
	return snapshot
}

func TestValidAcceptsUniqueEqualizerIDs(t *testing.T) {
	if !Valid(validEqualizerSnapshot()) {
		t.Fatal("unique equalizer bands were rejected")
	}
}

func TestValidRejectsDuplicateEqualizerIDs(t *testing.T) {
	snapshot := validEqualizerSnapshot()
	snapshot.Equalizer[9].ID = 1
	if Valid(snapshot) {
		t.Fatal("duplicate equalizer band IDs were accepted")
	}
}
