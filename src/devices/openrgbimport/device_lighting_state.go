package openrgbimport

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"LumenForge/src/common"
	"LumenForge/src/rgb"
)

const (
	deviceLightingStateSchemaVersion = 1
	defaultDeviceLightingEffect      = "static"
	defaultDeviceLightingBrightness  = uint8(100)
)

// DeviceLightingState is the target-level state that remains separate from
// complete effect customizations.
type DeviceLightingState struct {
	SelectedEffect string `json:"selectedEffect"`
	Brightness     uint8  `json:"brightness"`
}

type deviceLightingStateDocument struct {
	SchemaVersion int                            `json:"schemaVersion"`
	Devices       map[string]DeviceLightingState `json:"devices"`
}

type deviceLightingStateWriter interface {
	Write(path string, data []byte) error
}

type atomicDeviceLightingStateWriter struct{}

func (atomicDeviceLightingStateWriter) Write(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create OpenRGB device-lighting directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".openrgb-device-lighting-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary OpenRGB device-lighting file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary OpenRGB device-lighting permissions: %w", err)
	}
	if _, err = temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary OpenRGB device-lighting state: %w", err)
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary OpenRGB device-lighting state: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close temporary OpenRGB device-lighting state: %w", err)
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace OpenRGB device-lighting state: %w", err)
	}
	return nil
}

// DeviceLightingStateStore owns selected effect and independent Brightness for
// OpenRGB-imported devices. Missing records intentionally resolve to the clean
// installation defaults without creating persistence.
type DeviceLightingStateStore struct {
	mu      sync.RWMutex
	path    string
	writer  deviceLightingStateWriter
	devices map[string]DeviceLightingState
}

func LoadDeviceLightingStateStore(path string) (*DeviceLightingStateStore, error) {
	return loadDeviceLightingStateStore(path, atomicDeviceLightingStateWriter{})
}

func loadDeviceLightingStateStore(path string, writer deviceLightingStateWriter) (*DeviceLightingStateStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("OpenRGB device-lighting state path is empty")
	}
	document := deviceLightingStateDocument{}
	if err := decodeDeviceLightingStateFile(path, &document); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load OpenRGB device-lighting state %q: %w", path, err)
		}
		document = deviceLightingStateDocument{
			SchemaVersion: deviceLightingStateSchemaVersion,
			Devices:       make(map[string]DeviceLightingState),
		}
	} else if err := validateDeviceLightingStateDocument(document); err != nil {
		return nil, fmt.Errorf("load OpenRGB device-lighting state %q: %w", path, err)
	}
	return &DeviceLightingStateStore{
		path:    path,
		writer:  writer,
		devices: cloneDeviceLightingStates(document.Devices),
	}, nil
}

func DefaultDeviceLightingState() DeviceLightingState {
	return DeviceLightingState{
		SelectedEffect: defaultDeviceLightingEffect,
		Brightness:     defaultDeviceLightingBrightness,
	}
}

// Resolve returns a value copy and whether target has persisted state.
func (store *DeviceLightingStateStore) Resolve(deviceID string) (DeviceLightingState, bool, error) {
	if err := validateOpenRGBDeviceIdentity(deviceID); err != nil {
		return DeviceLightingState{}, false, err
	}
	if store == nil {
		return DeviceLightingState{}, false, fmt.Errorf("OpenRGB device-lighting state store is nil")
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	state, found := store.devices[deviceID]
	if !found {
		return DefaultDeviceLightingState(), false, nil
	}
	return state, true, nil
}

// Set atomically replaces one complete target-level state record.
func (store *DeviceLightingStateStore) Set(deviceID string, state DeviceLightingState) error {
	if err := validateOpenRGBDeviceIdentity(deviceID); err != nil {
		return err
	}
	if err := validateDeviceLightingState(state); err != nil {
		return err
	}
	if store == nil || store.writer == nil {
		return fmt.Errorf("OpenRGB device-lighting state store is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	next := cloneDeviceLightingStates(store.devices)
	next[deviceID] = state
	if err := store.persist(next); err != nil {
		return err
	}
	store.devices = next
	return nil
}

// Delete removes one target record. A missing target is a successful no-op
// and performs no write.
func (store *DeviceLightingStateStore) Delete(deviceID string) (bool, error) {
	if err := validateOpenRGBDeviceIdentity(deviceID); err != nil {
		return false, err
	}
	if store == nil || store.writer == nil {
		return false, fmt.Errorf("OpenRGB device-lighting state store is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, found := store.devices[deviceID]; !found {
		return false, nil
	}
	next := cloneDeviceLightingStates(store.devices)
	delete(next, deviceID)
	if err := store.persist(next); err != nil {
		return false, err
	}
	store.devices = next
	return true, nil
}

func (store *DeviceLightingStateStore) persist(devices map[string]DeviceLightingState) error {
	data, err := json.MarshalIndent(deviceLightingStateDocument{
		SchemaVersion: deviceLightingStateSchemaVersion,
		Devices:       devices,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode OpenRGB device-lighting state: %w", err)
	}
	data = append(data, '\n')
	if err = store.writer.Write(store.path, data); err != nil {
		return fmt.Errorf("persist OpenRGB device-lighting state: %w", err)
	}
	return nil
}

func validateDeviceLightingState(state DeviceLightingState) error {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(state.SelectedEffect)
	if !independentDeviceEffectDescriptor(descriptor, ok) {
		return fmt.Errorf("selected effect %q is not valid for an independent device", state.SelectedEffect)
	}
	if state.Brightness > 100 {
		return fmt.Errorf("brightness must be between 0 and 100")
	}
	return nil
}

func independentDeviceEffectDescriptor(descriptor rgb.SoftwareEffectDescriptor, found bool) bool {
	return found && descriptor.Scope.Includes(rgb.EffectScopeDevice)
}

func validateOpenRGBDeviceIdentity(deviceID string) error {
	if len(deviceID) > 512 || !common.AlphanumericDashRegex.MatchString(deviceID) {
		return fmt.Errorf("OpenRGB device identity is malformed")
	}
	return nil
}

func validateDeviceLightingStateDocument(document deviceLightingStateDocument) error {
	if document.SchemaVersion != deviceLightingStateSchemaVersion {
		return fmt.Errorf("unsupported schema version %d", document.SchemaVersion)
	}
	if document.Devices == nil {
		return fmt.Errorf("device state records are missing")
	}
	for deviceID, state := range document.Devices {
		if err := validateOpenRGBDeviceIdentity(deviceID); err != nil {
			return err
		}
		if err := validateDeviceLightingState(state); err != nil {
			return fmt.Errorf("device %q: %w", deviceID, err)
		}
	}
	return nil
}

func decodeDeviceLightingStateFile(path string, destination any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 8<<20))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(destination); err != nil {
		return err
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func cloneDeviceLightingStates(source map[string]DeviceLightingState) map[string]DeviceLightingState {
	cloned := make(map[string]DeviceLightingState, len(source))
	for deviceID, state := range source {
		cloned[deviceID] = state
	}
	return cloned
}
