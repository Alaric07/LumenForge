package flashtappresentation

type Key struct {
	KeyIndex int
	KeyName  string
	Eligible bool
	Selected bool
}
type SelectedSlot struct {
	SlotIndex int
	KeyIndex  int
}
type Option struct {
	Value int
	Label string
}
type Color struct {
	Red   float64
	Green float64
	Blue  float64
}
type Snapshot struct {
	Supported     bool
	Active        bool
	Mode          int
	Modes         []Option
	Keys          []Key
	SelectedSlots []SelectedSlot
	Color         Color
}
