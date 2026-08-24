package openrgbimport

import "LumenForge/src/lightingsettings"

type DeviceLightingState = lightingsettings.IndependentDeviceLightingState
type DeviceLightingStateStore = lightingsettings.IndependentDeviceStateStore

func DefaultDeviceLightingState() DeviceLightingState {
	return lightingsettings.DefaultIndependentDeviceLightingState()
}

func LoadDeviceLightingStateStore(path string) (*DeviceLightingStateStore, error) {
	return lightingsettings.LoadIndependentDeviceStateStore(path)
}

func validateDeviceLightingState(state DeviceLightingState) error {
	return lightingsettings.ValidateIndependentDeviceLightingState(state)
}
