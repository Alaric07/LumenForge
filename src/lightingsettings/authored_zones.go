package lightingsettings

import (
	"errors"
	"fmt"
)

// Authored palettes are device-specific colors, not generic Static settings or
// input indicators. Topology and shipped palette defaults stay in the device.
// They use the existing DeviceStore file and its atomic writer.
func (store *DeviceStore) GetAuthoredZones(deviceID, effect string) (map[string]Color, bool, error) {
	if err := validateAuthoredIdentity(deviceID, effect); err != nil {
		return nil, false, err
	}
	if store == nil {
		return nil, false, fmt.Errorf("device lighting settings store is unavailable")
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	colors, found := store.authoredZones[deviceID][effect]
	return cloneZoneColors(colors), found, nil
}

// SetAuthoredZones persists a complete authored palette before publishing it.
// The device validates the exact physical zone IDs before calling this seam.
func (store *DeviceStore) SetAuthoredZones(deviceID, effect string, colors map[string]Color) error {
	if err := validateAuthoredIdentity(deviceID, effect); err != nil {
		return err
	}
	if err := validateZoneColors(colors); err != nil {
		return err
	}
	if store == nil || store.writer == nil {
		return fmt.Errorf("device lighting settings store is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	next := cloneAuthoredZones(store.authoredZones)
	if next[deviceID] == nil {
		next[deviceID] = make(map[string]map[string]Color)
	}
	next[deviceID][effect] = cloneZoneColors(colors)
	if err := store.persist(deviceStoreDocument{SchemaVersion: storeSchemaVersion, Devices: store.devices, AuthoredZones: next}); err != nil {
		return err
	}
	store.authoredZones = next
	return nil
}

// ImportDevice initializes absent customizations and authored palettes together.
// commitState is the final, error-returning target-state write; it must not call
// this store. A failed target write restores the previous settings document and
// never publishes the candidate maps. This is a startup import, not a second
// authority or a general cross-file transaction system.
func (store *DeviceStore) ImportDevice(deviceID string, effects map[string]EffectSettings, palettes map[string]map[string]Color, commitState func() error) error {
	if err := validateDeviceIdentity(deviceID); err != nil {
		return err
	}
	for effect, value := range effects {
		if err := validateStoredSettings(effect, value); err != nil {
			return err
		}
	}
	if err := validateAuthoredZones(map[string]map[string]map[string]Color{deviceID: palettes}); err != nil {
		return err
	}
	if store == nil || store.writer == nil || commitState == nil {
		return fmt.Errorf("device lighting import is unavailable")
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	next, authored := cloneDeviceRecords(store.devices), cloneAuthoredZones(store.authoredZones)
	if next[deviceID] == nil {
		next[deviceID] = make(map[string]EffectSettings)
	}
	for effect, value := range effects {
		if _, found := next[deviceID][effect]; !found {
			next[deviceID][effect] = value.Clone()
		}
	}
	if authored[deviceID] == nil {
		authored[deviceID] = make(map[string]map[string]Color)
	}
	for effect, colors := range palettes {
		if _, found := authored[deviceID][effect]; !found {
			authored[deviceID][effect] = cloneZoneColors(colors)
		}
	}
	old := deviceStoreDocument{SchemaVersion: storeSchemaVersion, Devices: store.devices, AuthoredZones: store.authoredZones}
	if err := store.persist(deviceStoreDocument{SchemaVersion: storeSchemaVersion, Devices: next, AuthoredZones: authored}); err != nil {
		return err
	}
	if err := commitState(); err != nil {
		if restoreErr := store.persist(old); restoreErr != nil {
			return errors.Join(err, fmt.Errorf("restore settings after failed import: %w", restoreErr))
		}
		return err
	}
	store.devices, store.authoredZones = next, authored
	return nil
}

func validateAuthoredIdentity(deviceID, effect string) error {
	if err := validateDeviceIdentity(deviceID); err != nil {
		return err
	}
	// Only Mouse has a canonical authored-palette consumer so far. Other native
	// special modes retain their own contract until explicitly migrated.
	if effect != "mouse" {
		return fmt.Errorf("unsupported device-authored palette %q", effect)
	}
	return nil
}

func validateZoneColors(colors map[string]Color) error {
	if len(colors) == 0 || len(colors) > 1024 {
		return fmt.Errorf("complete authored zone colors are required")
	}
	for zone, color := range colors {
		if zone == "" || len(zone) > 128 {
			return fmt.Errorf("invalid authored zone identity")
		}
		if err := validateColor(color); err != nil {
			return err
		}
	}
	return nil
}

func validateAuthoredZones(devices map[string]map[string]map[string]Color) error {
	for id, effects := range devices {
		for effect, colors := range effects {
			if err := validateAuthoredIdentity(id, effect); err != nil {
				return err
			}
			if err := validateZoneColors(colors); err != nil {
				return err
			}
		}
	}
	return nil
}

func cloneZoneColors(colors map[string]Color) map[string]Color {
	copy := make(map[string]Color, len(colors))
	for zone, color := range colors {
		copy[zone] = color
	}
	return copy
}

func cloneAuthoredZones(devices map[string]map[string]map[string]Color) map[string]map[string]map[string]Color {
	copy := make(map[string]map[string]map[string]Color, len(devices))
	for id, effects := range devices {
		copy[id] = make(map[string]map[string]Color, len(effects))
		for effect, colors := range effects {
			copy[id][effect] = cloneZoneColors(colors)
		}
	}
	return copy
}
