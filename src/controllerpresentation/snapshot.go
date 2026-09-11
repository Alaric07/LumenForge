// Package controllerpresentation defines the read-only controller workspace contract.
package controllerpresentation

import (
	"LumenForge/src/common"
	"sort"
)

type AssignmentType struct {
	ID    uint8
	Label string
}
type Assignment struct {
	Index                                   int
	Label                                   string
	Default, ActionHold, IsMacro, OnRelease bool
	ActionType                              uint8
	ActionCommand                           uint16
}
type Vibration struct {
	Module uint8
	Label  string
	Value  uint8
}
type Thumbstick struct {
	Module                     uint8
	Label                      string
	Mode                       uint8
	SensitivityX, SensitivityY uint8
	InvertY                    bool
	Modes                      []AssignmentType
}
type CurvePoint struct {
	Index int
	X, Y  uint8
}
type Analog struct {
	ID                       uint8
	Label                    string
	DeadZoneMin, DeadZoneMax uint8
	Points                   []CurvePoint
}
type AnalogData struct {
	DeadZoneMin, DeadZoneMax uint8
	Points                   map[int]common.CurveData
}
type Snapshot struct {
	Assignments     []Assignment
	AssignmentTypes []AssignmentType
	Vibrations      []Vibration
	Thumbsticks     []Thumbstick
	Analogs         []Analog
}

var assignmentTypeIDs = []uint8{0, 1, 2, 3, 4, 8, 9, 10}
var analogLabels = []string{"Left Thumbstick", "Right Thumbstick", "Left Trigger", "Right Trigger"}

func expectedTypes(values map[int]string) ([]AssignmentType, bool) {
	if len(values) != len(assignmentTypeIDs) {
		return nil, false
	}
	out := make([]AssignmentType, 0, len(assignmentTypeIDs))
	for _, id := range assignmentTypeIDs {
		label, ok := values[int(id)]
		if !ok || label == "" {
			return nil, false
		}
		out = append(out, AssignmentType{ID: id, Label: label})
	}
	return out, true
}

// Build returns false unless every controller capability is complete and safe to render.
func Build(assignments map[int]Assignment, assignmentTypes map[int]string, vibration [2]uint8, sticks [2]Thumbstick, analog map[int]AnalogData) (Snapshot, bool) {
	types, ok := expectedTypes(assignmentTypes)
	if !ok || len(assignments) == 0 {
		return Snapshot{}, false
	}
	s := Snapshot{AssignmentTypes: types, Assignments: make([]Assignment, 0, len(assignments))}
	for index, assignment := range assignments {
		if index < 0 || assignment.Index != index || assignment.Label == "" || !containsType(types, assignment.ActionType) {
			return Snapshot{}, false
		}
		s.Assignments = append(s.Assignments, assignment)
	}
	sort.Slice(s.Assignments, func(i, j int) bool { return s.Assignments[i].Index < s.Assignments[j].Index })
	for module, value := range vibration {
		if value > 100 {
			return Snapshot{}, false
		}
		label := "Left Vibration"
		if module == 1 {
			label = "Right Vibration"
		}
		s.Vibrations = append(s.Vibrations, Vibration{Module: uint8(module), Label: label, Value: value})
	}
	for module, stick := range sticks {
		if stick.Module != uint8(module) || stick.Label == "" || stick.Mode > 2 || stick.SensitivityX < 5 || stick.SensitivityX > 50 || stick.SensitivityY < 5 || stick.SensitivityY > 50 || len(stick.Modes) != 3 {
			return Snapshot{}, false
		}
		for id := uint8(0); id < 3; id++ {
			if stick.Modes[id].ID != id || stick.Modes[id].Label == "" {
				return Snapshot{}, false
			}
		}
		s.Thumbsticks = append(s.Thumbsticks, stick)
	}
	if len(analog) != len(analogLabels) {
		return Snapshot{}, false
	}
	for id, label := range analogLabels {
		data, ok := analog[id]
		if !ok || data.DeadZoneMin < 2 || data.DeadZoneMin > 15 || data.DeadZoneMax < 2 || data.DeadZoneMax > 15 || data.DeadZoneMin > data.DeadZoneMax || len(data.Points) == 0 {
			return Snapshot{}, false
		}
		item := Analog{ID: uint8(id), Label: label, DeadZoneMin: data.DeadZoneMin, DeadZoneMax: data.DeadZoneMax}
		for pointIndex := 0; pointIndex < len(data.Points); pointIndex++ {
			point, ok := data.Points[pointIndex]
			if !ok || point.X > 100 || point.Y > 100 {
				return Snapshot{}, false
			}
			item.Points = append(item.Points, CurvePoint{Index: pointIndex, X: point.X, Y: point.Y})
		}
		s.Analogs = append(s.Analogs, item)
	}
	return s, true
}

func containsType(types []AssignmentType, value uint8) bool {
	for _, kind := range types {
		if kind.ID == value {
			return true
		}
	}
	return false
}
