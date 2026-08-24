package scimitarprorgb

import "LumenForge/src/rgb"

type scimitarLightingController struct {
	adapter    scimitarLightingAdapter
	ownership  scimitarLightingOwnership
	writeFrame func([]byte)
	activeRGB  **rgb.ActiveRGB
}

func newScimitarLightingController(
	adapter scimitarLightingAdapter,
	ownership scimitarLightingOwnership,
	writeFrame func([]byte),
	activeRGB **rgb.ActiveRGB,
) scimitarLightingController {
	return scimitarLightingController{
		adapter:    adapter,
		ownership:  ownership,
		writeFrame: writeFrame,
		activeRGB:  activeRGB,
	}
}

func (c scimitarLightingController) writeLocalFrame(
	logicalFrame scimitarLogicalFrame,
	dpiColor rgb.Color,
) bool {
	if !c.ownership.allowsLocalRendering() {
		return false
	}
	c.writeFrame(c.adapter.composeScimitarHardwareFrame(logicalFrame, dpiColor))
	return true
}

func (c scimitarLightingController) start() *rgb.ActiveRGB {
	activeRGB := rgb.Exit()
	*c.activeRGB = activeRGB
	return activeRGB
}

func (c scimitarLightingController) stop() {
	if c.activeRGB == nil || *c.activeRGB == nil {
		return
	}
	(*c.activeRGB).Exit <- true
	*c.activeRGB = nil
}
