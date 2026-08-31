package lightingsettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"LumenForge/src/common"
	"LumenForge/src/rgb"
)

const (
	independentDeviceStateSchemaVersion = 1

	// DefaultIndependentDeviceEffect is the non-persisted clean-install effect.
	DefaultIndependentDeviceEffect = "static"
	// DefaultIndependentDeviceBrightness is the non-persisted clean-install Brightness.
	DefaultIndependentDeviceBrightness = uint8(100)
)

// IndependentDeviceLightingState is target-level state kept separately from
// complete effect customizations.
type IndependentDeviceLightingState struct {
	SelectedEffect string `json:"selectedEffect"`
	Brightness     uint8  `json:"brightness"`
}

type independentDeviceStateDocument struct {
	SchemaVersion int                                       `json:"schemaVersion"`
	Devices       map[string]IndependentDeviceLightingState `json:"devices"`
}

// IndependentDeviceStateAccess is the target-state surface shared by
// independent-device lighting consumers.
type IndependentDeviceStateAccess interface {
	Resolve(string) (IndependentDeviceLightingState, bool, error)
	Set(string, IndependentDeviceLightingState) error
	Delete(string) (bool, error)
}

type independentDeviceStateWriter interface {
	Write(path string, data []byte) error
}

type atomicIndependentDeviceStateWriter struct{}

func (atomicIndependentDeviceStateWriter) Write(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create independent-device lighting directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".independent-device-lighting-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary independent-device lighting file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary independent-device lighting permissions: %w", err)
	}
	if _, err = temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary independent-device lighting state: %w", err)
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary independent-device lighting state: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close temporary independent-device lighting state: %w", err)
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace independent-device lighting state: %w", err)
	}
	return nil
}

// IndependentDeviceStateStore owns selected effect and Brightness for
// independent devices. Missing records resolve to clean-install defaults
// without creating persistence.
type IndependentDeviceStateStore struct {
	mu      sync.RWMutex
	path    string
	writer  independentDeviceStateWriter
	devices map[string]IndependentDeviceLightingState
}

// LoadIndependentDeviceStateStore loads the independent-device target-state
// store. A missing file is a valid empty store and is not created by loading.
func LoadIndependentDeviceStateStore(path string) (*IndependentDeviceStateStore, error) {
	return loadIndependentDeviceStateStore(path, atomicIndependentDeviceStateWriter{})
}

// Path returns the resolved persistence path owned by the store.
func (store *IndependentDeviceStateStore) Path() string {
	if store == nil {
		return ""
	}
	return store.path
}

func loadIndependentDeviceStateStore(path string, writer independentDeviceStateWriter) (*IndependentDeviceStateStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("independent-device lighting state path is empty")
	}
	document := independentDeviceStateDocument{}
	if err := decodeIndependentDeviceStateFile(path, &document); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load independent-device lighting state %q: %w", path, err)
		}
		document = independentDeviceStateDocument{
			SchemaVersion: independentDeviceStateSchemaVersion,
			Devices:       make(map[string]IndependentDeviceLightingState),
		}
	} else if err := validateIndependentDeviceStateDocument(document); err != nil {
		return nil, fmt.Errorf("load independent-device lighting state %q: %w", path, err)
	} else {
		document.Devices = sanitizeIndependentDeviceStates(document.Devices)
	}
	return &IndependentDeviceStateStore{
		path:    path,
		writer:  writer,
		devices: cloneIndependentDeviceStates(document.Devices),
	}, nil
}

// DefaultIndependentDeviceLightingState returns the non-persisted state used
// when an independent device has no target-state record.
func DefaultIndependentDeviceLightingState() IndependentDeviceLightingState {
	return IndependentDeviceLightingState{
		SelectedEffect: DefaultIndependentDeviceEffect,
		Brightness:     DefaultIndependentDeviceBrightness,
	}
}

// Resolve returns a value copy and whether the target has persisted state.
func (store *IndependentDeviceStateStore) Resolve(deviceID string) (IndependentDeviceLightingState, bool, error) {
	if err := validateIndependentDeviceIdentity(deviceID); err != nil {
		return IndependentDeviceLightingState{}, false, err
	}
	if store == nil {
		return IndependentDeviceLightingState{}, false, fmt.Errorf("independent-device lighting state store is nil")
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	state, found := store.devices[deviceID]
	if !found {
		return DefaultIndependentDeviceLightingState(), false, nil
	}
	return state, true, nil
}

// Set atomically replaces one complete target-level state record.
func (store *IndependentDeviceStateStore) Set(deviceID string, state IndependentDeviceLightingState) error {
	if err := validateIndependentDeviceIdentity(deviceID); err != nil {
		return err
	}
	if err := ValidateIndependentDeviceLightingState(state); err != nil {
		return err
	}
	if store == nil || store.writer == nil {
		return fmt.Errorf("independent-device lighting state store is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	next := cloneIndependentDeviceStates(store.devices)
	next[deviceID] = state
	if err := store.persist(next); err != nil {
		return err
	}
	store.devices = next
	return nil
}

// Delete removes one target record. A missing target is a successful no-op
// and performs no write.
func (store *IndependentDeviceStateStore) Delete(deviceID string) (bool, error) {
	if err := validateIndependentDeviceIdentity(deviceID); err != nil {
		return false, err
	}
	if store == nil || store.writer == nil {
		return false, fmt.Errorf("independent-device lighting state store is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, found := store.devices[deviceID]; !found {
		return false, nil
	}
	next := cloneIndependentDeviceStates(store.devices)
	delete(next, deviceID)
	if err := store.persist(next); err != nil {
		return false, err
	}
	store.devices = next
	return true, nil
}

func (store *IndependentDeviceStateStore) persist(devices map[string]IndependentDeviceLightingState) error {
	data, err := json.MarshalIndent(independentDeviceStateDocument{
		SchemaVersion: independentDeviceStateSchemaVersion,
		Devices:       devices,
	}, "", "  ")
	if err != nil {
		return fmt.Errorf("encode independent-device lighting state: %w", err)
	}
	data = append(data, '\n')
	if err = store.writer.Write(store.path, data); err != nil {
		return fmt.Errorf("persist independent-device lighting state: %w", err)
	}
	return nil
}

// ValidateIndependentDeviceLightingState verifies one complete target-state
// record without reading or writing persistence.
func ValidateIndependentDeviceLightingState(state IndependentDeviceLightingState) error {
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(state.SelectedEffect)
	if !independentDeviceEffectDescriptor(descriptor, ok) && !specialIndependentDeviceEffectID(state.SelectedEffect) {
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

func specialIndependentDeviceEffectID(effect string) bool {
	// Device-specific effects are selected through the independent-device state
	// store but deliberately have no shared software-effect descriptor/settings.
	return effect == "mousepad" || effect == "mouse" || effect == "keyboard" || effect == "probe-temperature"
}

func validateIndependentDeviceIdentity(deviceID string) error {
	if len(deviceID) > 512 || !common.AlphanumericDashRegex.MatchString(deviceID) {
		return fmt.Errorf("independent-device identity is malformed")
	}
	return nil
}

func validateIndependentDeviceStateDocument(document independentDeviceStateDocument) error {
	if document.SchemaVersion != independentDeviceStateSchemaVersion {
		return fmt.Errorf("unsupported schema version %d", document.SchemaVersion)
	}
	if document.Devices == nil {
		return fmt.Errorf("device state records are missing")
	}
	return nil
}

func sanitizeIndependentDeviceStates(devices map[string]IndependentDeviceLightingState) map[string]IndependentDeviceLightingState {
	sanitized := make(map[string]IndependentDeviceLightingState, len(devices))
	for deviceID, state := range devices {
		if err := validateIndependentDeviceIdentity(deviceID); err != nil {
			log.Printf("discarding independent-device lighting state for %q: %v", deviceID, err)
			continue
		}
		if err := ValidateIndependentDeviceLightingState(state); err != nil {
			log.Printf("discarding independent-device lighting state for %q: %v", deviceID, err)
			continue
		}
		sanitized[deviceID] = state
	}
	return sanitized
}

func decodeIndependentDeviceStateFile(path string, destination any) error {
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

func cloneIndependentDeviceStates(source map[string]IndependentDeviceLightingState) map[string]IndependentDeviceLightingState {
	cloned := make(map[string]IndependentDeviceLightingState, len(source))
	for deviceID, state := range source {
		cloned[deviceID] = state
	}
	return cloned
}
