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
	Equalizer         []EqualizerBand
	Muted             *bool
	MuteIndicator     *SelectSetting
	NoiseCancellation *SelectSetting
	Sidetone          *SidetoneSetting
	Wheels            []WheelSetting
	Assignments       []Assignment
}

// SelectSetting is an optional device-owned select control.
type SelectSetting struct {
	Value   int
	Options []SelectOption
}

// SidetoneSetting is the complete legacy sidetone contract.
type SidetoneSetting struct {
	Value      int
	Options    []SelectOption
	ValueRange *RangedSetting
}

// WheelSetting is one independently configurable headset wheel.
type WheelSetting struct {
	ID      uint8
	Label   string
	Setting SelectSetting
}

// RangedSetting is a persisted integer setting with its complete range.
type RangedSetting struct{ Value, Minimum, Maximum, Step int }

// Assignment is one device-owned input assignment without keyboard geometry.
type Assignment struct {
	ID                                      int
	Label                                   string
	Default, ActionHold, IsMacro, OnRelease bool
	ActionType                              uint8
	ActionCommand                           uint16
	Types                                   []AssignmentType
}
type AssignmentType struct {
	ID    uint8
	Label string
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
	validSelectSetting := func(setting *SelectSetting) bool {
		if setting == nil || len(setting.Options) == 0 {
			return false
		}
		found := false
		for _, option := range setting.Options {
			if option.Label == "" {
				return false
			}
			found = found || option.Value == setting.Value
		}
		return found
	}
	if setting := snapshot.MuteIndicator; setting != nil {
		if !validSelectSetting(setting) {
			return false
		}
	}
	if setting := snapshot.NoiseCancellation; setting != nil && !validSelectSetting(setting) {
		return false
	}
	if setting := snapshot.Sidetone; setting != nil {
		if len(setting.Options) == 0 || setting.ValueRange == nil || setting.ValueRange.Minimum > setting.ValueRange.Maximum || setting.ValueRange.Step <= 0 {
			return false
		}
		found := false
		for _, option := range setting.Options {
			if option.Label == "" {
				return false
			}
			found = found || option.Value == setting.Value
		}
		if !found || setting.ValueRange.Value < setting.ValueRange.Minimum || setting.ValueRange.Value > setting.ValueRange.Maximum {
			return false
		}
	}
	seenWheels := make(map[uint8]struct{}, len(snapshot.Wheels))
	for _, wheel := range snapshot.Wheels {
		if wheel.ID == 0 || wheel.Label == "" || !validSelectSetting(&wheel.Setting) {
			return false
		}
		if _, duplicate := seenWheels[wheel.ID]; duplicate {
			return false
		}
		seenWheels[wheel.ID] = struct{}{}
	}
	for _, assignment := range snapshot.Assignments {
		if assignment.ID < 0 || assignment.Label == "" || len(assignment.Types) == 0 {
			return false
		}
		found := false
		for _, kind := range assignment.Types {
			if kind.Label == "" {
				return false
			}
			found = found || kind.ID == assignment.ActionType
		}
		if !found {
			return false
		}
	}
	return true
}
