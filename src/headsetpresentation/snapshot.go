// Package headsetpresentation defines capability-gated headset data for the
// shared Devices workspace.
package headsetpresentation

// EqualizerBand is one persisted headset equalizer value.
type EqualizerBand struct {
	ID    int
	Label string
	Value float64
}

// SelectOption is one persisted headset setting option.
type SelectOption struct {
	Value int
	Label string
}

// Snapshot contains only headset capabilities whose complete legacy contract
// is available to the shared workspace.
type Snapshot struct {
	Equalizer     []EqualizerBand
	Muted         *bool
	MuteIndicator *SelectSetting
}

// SelectSetting is an optional device-owned select control.
type SelectSetting struct {
	Value   int
	Options []SelectOption
}

// Valid returns whether a snapshot contains only complete, usable controls.
func Valid(snapshot Snapshot) bool {
	if len(snapshot.Equalizer) != 10 {
		return false
	}
	for _, band := range snapshot.Equalizer {
		if band.ID < 1 || band.Label == "" {
			return false
		}
	}
	if setting := snapshot.MuteIndicator; setting != nil {
		if len(setting.Options) == 0 {
			return false
		}
		found := false
		for _, option := range setting.Options {
			if option.Label == "" {
				return false
			}
			found = found || option.Value == setting.Value
		}
		if !found {
			return false
		}
	}
	return true
}
