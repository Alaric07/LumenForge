// Package displaypresentation defines controller-neutral LCD workspace data.
package displaypresentation

// Option is one backend-advertised display setting.
type Option struct {
	ID       int
	Label    string
	Selected bool
}

// ImageOption is one backend-advertised LCD image.
type ImageOption struct {
	Name     string
	Selected bool
}

// Snapshot is a read-only view of an optional device display. Mutations remain
// on the existing LCD routes and controller methods.
type Snapshot struct {
	Available          bool
	ChannelID          int
	SelectedMode       int
	Modes              []Option
	SelectedRotation   int
	Rotations          []Option
	SelectedBrightness int
	BrightnessLevels   []Option
	SelectedImage      string
	Images             []ImageOption
	ImageMode          bool
	ImageModeID        int
}
