package scimitarrgbelite

import "LumenForge/src/rgb"

type scimitarEliteLightingController struct {
	adapter    scimitarEliteLightingAdapter
	ownership  scimitarEliteLightingOwnership
	writeFrame func([]byte)
	activeRGB  **rgb.ActiveRGB
}

func newScimitarEliteLightingController(
	adapter scimitarEliteLightingAdapter,
	ownership scimitarEliteLightingOwnership,
	writeFrame func([]byte),
	activeRGB **rgb.ActiveRGB,
) scimitarEliteLightingController {
	return scimitarEliteLightingController{
		adapter:    adapter,
		ownership:  ownership,
		writeFrame: writeFrame,
		activeRGB:  activeRGB,
	}
}

func (c scimitarEliteLightingController) writeLocalFrame(
	logicalFrame scimitarEliteLogicalFrame,
	dpiColor rgb.Color,
) bool {
	if !c.ownership.allowsLocalRendering() {
		return false
	}
	c.writeFrame(c.adapter.composeScimitarEliteHardwareFrame(logicalFrame, dpiColor))
	return true
}

func (c scimitarEliteLightingController) start() *rgb.ActiveRGB {
	activeRGB := rgb.Exit()
	*c.activeRGB = activeRGB
	return activeRGB
}

func (c scimitarEliteLightingController) stop() {
	if c.activeRGB == nil || *c.activeRGB == nil {
		return
	}
	(*c.activeRGB).Exit <- true
	*c.activeRGB = nil
}
