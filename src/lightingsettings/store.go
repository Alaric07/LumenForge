package lightingsettings

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode/utf8"

	"LumenForge/src/rgb"
)

const storeSchemaVersion = 1

type persistenceWriter interface {
	Write(path string, data []byte) error
}

type atomicPersistenceWriter struct{}

func (atomicPersistenceWriter) Write(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create lighting settings directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".lighting-settings-*.tmp")
	if err != nil {
		return fmt.Errorf("create temporary lighting settings file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)

	if err = temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set temporary lighting settings permissions: %w", err)
	}
	if _, err = temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write temporary lighting settings: %w", err)
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync temporary lighting settings: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close temporary lighting settings: %w", err)
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace lighting settings: %w", err)
	}
	return nil
}

type deviceStoreDocument struct {
	SchemaVersion int                                  `json:"schemaVersion"`
	Devices       map[string]map[string]EffectSettings `json:"devices"`
}

// DeviceStore owns independent-device effect customizations.
type DeviceStore struct {
	mu      sync.RWMutex
	path    string
	writer  persistenceWriter
	devices map[string]map[string]EffectSettings
}

// LoadDeviceStore loads the dedicated independent-device customization store.
// A missing file is a valid empty store and is not created by loading.
func LoadDeviceStore(path string) (*DeviceStore, error) {
	return loadDeviceStore(path, atomicPersistenceWriter{})
}

func loadDeviceStore(path string, writer persistenceWriter) (*DeviceStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("device lighting settings path is empty")
	}
	var document deviceStoreDocument
	if err := decodeStoreFile(path, &document); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load device lighting settings %q: %w", path, err)
		}
		document = deviceStoreDocument{SchemaVersion: storeSchemaVersion, Devices: make(map[string]map[string]EffectSettings)}
	} else if err := validateDeviceDocument(document); err != nil {
		return nil, fmt.Errorf("load device lighting settings %q: %w", path, err)
	}
	return &DeviceStore{path: path, writer: writer, devices: cloneDeviceRecords(document.Devices)}, nil
}

// Get returns a defensive copy and whether the device/effect customization exists.
func (store *DeviceStore) Get(deviceID, effectID string) (EffectSettings, bool, error) {
	if err := validateDeviceIdentity(deviceID); err != nil {
		return EffectSettings{}, false, err
	}
	if err := validateEffectIdentity(effectID); err != nil {
		return EffectSettings{}, false, err
	}
	if store == nil {
		return EffectSettings{}, false, fmt.Errorf("device lighting settings store is nil")
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	settings, ok := store.devices[deviceID][effectID]
	if !ok {
		return EffectSettings{}, false, nil
	}
	return settings.Clone(), true, nil
}

// Set atomically replaces one complete device/effect customization.
func (store *DeviceStore) Set(deviceID, effectID string, settings EffectSettings) error {
	if err := validateDeviceIdentity(deviceID); err != nil {
		return err
	}
	if err := validateStoredSettings(effectID, settings); err != nil {
		return err
	}
	if store == nil || store.writer == nil {
		return fmt.Errorf("device lighting settings store is unavailable")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	next := cloneDeviceRecords(store.devices)
	if next[deviceID] == nil {
		next[deviceID] = make(map[string]EffectSettings)
	}
	next[deviceID][effectID] = settings.Clone()
	if err := store.persist(deviceStoreDocument{SchemaVersion: storeSchemaVersion, Devices: next}); err != nil {
		return err
	}
	store.devices = next
	return nil
}

// Delete removes only one device/effect customization. A missing record is a
// successful no-op and performs no write.
func (store *DeviceStore) Delete(deviceID, effectID string) (bool, error) {
	if err := validateDeviceIdentity(deviceID); err != nil {
		return false, err
	}
	if err := validateEffectIdentity(effectID); err != nil {
		return false, err
	}
	if store == nil || store.writer == nil {
		return false, fmt.Errorf("device lighting settings store is unavailable")
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.devices[deviceID][effectID]; !ok {
		return false, nil
	}
	next := cloneDeviceRecords(store.devices)
	delete(next[deviceID], effectID)
	if len(next[deviceID]) == 0 {
		delete(next, deviceID)
	}
	if err := store.persist(deviceStoreDocument{SchemaVersion: storeSchemaVersion, Devices: next}); err != nil {
		return false, err
	}
	store.devices = next
	return true, nil
}

func (store *DeviceStore) persist(document deviceStoreDocument) error {
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode device lighting settings: %w", err)
	}
	data = append(data, '\n')
	if err = store.writer.Write(store.path, data); err != nil {
		return fmt.Errorf("persist device lighting settings: %w", err)
	}
	return nil
}

type clusterStoreDocument struct {
	SchemaVersion int                       `json:"schemaVersion"`
	Effects       map[string]EffectSettings `json:"effects"`
}

// ClusterStore owns customizations for the one established RGB Cluster target.
type ClusterStore struct {
	mu      sync.RWMutex
	path    string
	writer  persistenceWriter
	effects map[string]EffectSettings
}

// LoadClusterStore loads the dedicated RGB Cluster customization store. A
// missing file is a valid empty store and is not created by loading.
func LoadClusterStore(path string) (*ClusterStore, error) {
	return loadClusterStore(path, atomicPersistenceWriter{})
}

func loadClusterStore(path string, writer persistenceWriter) (*ClusterStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("cluster lighting settings path is empty")
	}
	var document clusterStoreDocument
	if err := decodeStoreFile(path, &document); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load cluster lighting settings %q: %w", path, err)
		}
		document = clusterStoreDocument{SchemaVersion: storeSchemaVersion, Effects: make(map[string]EffectSettings)}
	} else if err := validateClusterDocument(document); err != nil {
		return nil, fmt.Errorf("load cluster lighting settings %q: %w", path, err)
	}
	return &ClusterStore{path: path, writer: writer, effects: cloneEffectRecords(document.Effects)}, nil
}

// Get returns a defensive copy and whether the cluster/effect customization exists.
func (store *ClusterStore) Get(effectID string) (EffectSettings, bool, error) {
	if err := validateEffectIdentity(effectID); err != nil {
		return EffectSettings{}, false, err
	}
	if store == nil {
		return EffectSettings{}, false, fmt.Errorf("cluster lighting settings store is nil")
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	settings, ok := store.effects[effectID]
	if !ok {
		return EffectSettings{}, false, nil
	}
	return settings.Clone(), true, nil
}

// Set atomically replaces one complete cluster/effect customization.
func (store *ClusterStore) Set(effectID string, settings EffectSettings) error {
	if err := validateStoredSettings(effectID, settings); err != nil {
		return err
	}
	if store == nil || store.writer == nil {
		return fmt.Errorf("cluster lighting settings store is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	next := cloneEffectRecords(store.effects)
	next[effectID] = settings.Clone()
	if err := store.persist(clusterStoreDocument{SchemaVersion: storeSchemaVersion, Effects: next}); err != nil {
		return err
	}
	store.effects = next
	return nil
}

// Delete removes only one cluster/effect customization. A missing record is a
// successful no-op and performs no write.
func (store *ClusterStore) Delete(effectID string) (bool, error) {
	if err := validateEffectIdentity(effectID); err != nil {
		return false, err
	}
	if store == nil || store.writer == nil {
		return false, fmt.Errorf("cluster lighting settings store is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if _, ok := store.effects[effectID]; !ok {
		return false, nil
	}
	next := cloneEffectRecords(store.effects)
	delete(next, effectID)
	if err := store.persist(clusterStoreDocument{SchemaVersion: storeSchemaVersion, Effects: next}); err != nil {
		return false, err
	}
	store.effects = next
	return true, nil
}

func (store *ClusterStore) persist(document clusterStoreDocument) error {
	data, err := json.MarshalIndent(document, "", "  ")
	if err != nil {
		return fmt.Errorf("encode cluster lighting settings: %w", err)
	}
	data = append(data, '\n')
	if err = store.writer.Write(store.path, data); err != nil {
		return fmt.Errorf("persist cluster lighting settings: %w", err)
	}
	return nil
}

func decodeStoreFile(path string, destination any) error {
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

func validateDeviceDocument(document deviceStoreDocument) error {
	if document.SchemaVersion != storeSchemaVersion {
		return fmt.Errorf("unsupported device store schema version %d", document.SchemaVersion)
	}
	if document.Devices == nil {
		return fmt.Errorf("device customization records are missing")
	}
	for deviceID, effects := range document.Devices {
		if err := validateDeviceIdentity(deviceID); err != nil {
			return err
		}
		for effectID, settings := range effects {
			if err := validateStoredSettings(effectID, settings); err != nil {
				return err
			}
		}
	}
	return nil
}

func validateClusterDocument(document clusterStoreDocument) error {
	if document.SchemaVersion != storeSchemaVersion {
		return fmt.Errorf("unsupported cluster store schema version %d", document.SchemaVersion)
	}
	if document.Effects == nil {
		return fmt.Errorf("cluster customization records are missing")
	}
	for effectID, settings := range document.Effects {
		if err := validateStoredSettings(effectID, settings); err != nil {
			return err
		}
	}
	return nil
}

func validateStoredSettings(effectID string, settings EffectSettings) error {
	if err := validateEffectIdentity(effectID); err != nil {
		return err
	}
	if settings.EffectID != effectID {
		return invalidSettings(settings.EffectID, "record effect does not match key %q", effectID)
	}
	return Validate(settings)
}

func validateEffectIdentity(effectID string) error {
	if _, ok := rgb.SoftwareEffectDescriptorByID(effectID); !ok {
		return fmt.Errorf("%w: %q", ErrUnknownEffect, effectID)
	}
	return nil
}

func validateDeviceIdentity(deviceID string) error {
	if deviceID == "" || len(deviceID) > 512 || !utf8.ValidString(deviceID) || strings.TrimSpace(deviceID) != deviceID {
		return fmt.Errorf("%w: independent-device identity is malformed", ErrInvalidTarget)
	}
	for _, value := range deviceID {
		if value < ' ' || value == 0x7f {
			return fmt.Errorf("%w: independent-device identity is malformed", ErrInvalidTarget)
		}
	}
	return nil
}

func cloneDeviceRecords(source map[string]map[string]EffectSettings) map[string]map[string]EffectSettings {
	cloned := make(map[string]map[string]EffectSettings, len(source))
	for deviceID, effects := range source {
		cloned[deviceID] = cloneEffectRecords(effects)
	}
	return cloned
}

func cloneEffectRecords(source map[string]EffectSettings) map[string]EffectSettings {
	cloned := make(map[string]EffectSettings, len(source))
	for effectID, settings := range source {
		cloned[effectID] = settings.Clone()
	}
	return cloned
}
