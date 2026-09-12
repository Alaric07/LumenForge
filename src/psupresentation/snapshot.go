// Package psupresentation defines the shared Devices workspace snapshot for
// Corsair PSUs, independent of their HID or Link-dongle transport.
package psupresentation

type FanMode struct {
	Value int
	Label string
}

type Fan struct {
	RPM     string
	Mode    int
	Options []FanMode
}

type Temperature struct {
	Label string
	Value string
}

type Rail struct {
	Label string
	Watts string
	Amps  string
	Volts string
}

type Snapshot struct {
	DeviceID     string
	Fan          Fan
	Temperatures []Temperature
	PowerOut     string
	Rails        []Rail
}
