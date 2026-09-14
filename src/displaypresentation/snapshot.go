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

// Display is one independently configured physical display. It retains the
// channel key required by the existing generic LCD mutation routes.
type Display struct {
	ChannelID          int
	Name               string
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
	// Displays is populated when a controller has more than one independently
	// configurable LCD. Legacy single-display providers continue to use the
	// fields above.
	Displays []Display
}
