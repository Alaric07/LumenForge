// Package linkpresentation defines source-backed connected LINK device data.
package linkpresentation

type Option struct {
	ID    int
	Label string
}

type CommanderDuoOverride struct {
	Enabled     bool
	LEDChannels uint8
}

type Device struct {
	ChannelID                                     int
	Position                                      string
	Name                                          string
	DeviceID                                      string
	Description                                   string
	ContainsPump, AIO, HasSpeed, HasLCD, TitanAIO bool
	AdapterOptions                                []Option
	SelectedAdapter                               int
	CommanderDuo                                  *CommanderDuoOverride
}

type Snapshot struct {
	DeviceID string
	Devices  []Device
}
