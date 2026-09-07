package keyactuationpresentation

type Key struct {
	KeyIndex                      int
	KeyName                       string
	Supported                     bool
	ActuationPoint                byte
	ActuationResetPoint           byte
	EnableActuationPointReset     bool
	EnableSecondaryActuationPoint bool
	SecondaryActuationPoint       byte
	SecondaryActuationResetPoint  byte
}
type Snapshot struct {
	Supported           bool
	MinValue            byte
	MaxValue            byte
	SecondaryMinimumGap byte
	Keys                []Key
}
