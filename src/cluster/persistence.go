package cluster

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"LumenForge/src/rgb"
)

const clusterPersistenceSchemaVersion = 1

type clusterPersistenceWriter interface {
	Write(string, []byte) error
}

type atomicClusterPersistenceWriter struct{}

func (atomicClusterPersistenceWriter) Write(path string, data []byte) error {
	directory := filepath.Dir(path)
	if err := os.MkdirAll(directory, 0o700); err != nil {
		return fmt.Errorf("create RGB Cluster persistence directory: %w", err)
	}
	temporary, err := os.CreateTemp(directory, ".rgb-cluster-*.tmp")
	if err != nil {
		return fmt.Errorf("create RGB Cluster temporary file: %w", err)
	}
	temporaryPath := temporary.Name()
	defer os.Remove(temporaryPath)
	if err = temporary.Chmod(0o600); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("set RGB Cluster temporary permissions: %w", err)
	}
	if _, err = temporary.Write(data); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("write RGB Cluster temporary file: %w", err)
	}
	if err = temporary.Sync(); err != nil {
		_ = temporary.Close()
		return fmt.Errorf("sync RGB Cluster temporary file: %w", err)
	}
	if err = temporary.Close(); err != nil {
		return fmt.Errorf("close RGB Cluster temporary file: %w", err)
	}
	if err = os.Rename(temporaryPath, path); err != nil {
		return fmt.Errorf("replace RGB Cluster persistence file: %w", err)
	}
	return nil
}

type clusterLightingState struct {
	SchemaVersion  int    `json:"schemaVersion"`
	SelectedEffect string `json:"selectedEffect"`
	Brightness     uint8  `json:"brightness"`
}

func defaultClusterLightingState() clusterLightingState {
	return clusterLightingState{SchemaVersion: clusterPersistenceSchemaVersion, SelectedEffect: "rainbow", Brightness: 100}
}

type clusterLightingStateStore struct {
	mu     sync.RWMutex
	path   string
	writer clusterPersistenceWriter
	state  clusterLightingState
}

func loadClusterLightingStateStore(path string) (*clusterLightingStateStore, error) {
	return loadClusterLightingStateStoreWithWriter(path, atomicClusterPersistenceWriter{})
}

func loadClusterLightingStateStoreWithWriter(path string, writer clusterPersistenceWriter) (*clusterLightingStateStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("RGB Cluster lighting state path is empty")
	}
	state := defaultClusterLightingState()
	if err := decodeClusterPersistence(path, &state); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load RGB Cluster lighting state %q: %w", path, err)
		}
	} else if err = validateClusterLightingState(state); err != nil {
		return nil, fmt.Errorf("load RGB Cluster lighting state %q: %w", path, err)
	}
	return &clusterLightingStateStore{path: path, writer: writer, state: state}, nil
}

func validateClusterLightingState(state clusterLightingState) error {
	if state.SchemaVersion != clusterPersistenceSchemaVersion {
		return fmt.Errorf("unsupported schema version %d", state.SchemaVersion)
	}
	descriptor, ok := rgb.SoftwareEffectDescriptorByID(state.SelectedEffect)
	if !ok || !descriptor.Scope.Includes(rgb.EffectScopeCluster) {
		return fmt.Errorf("selected effect %q is not supported by RGB Cluster", state.SelectedEffect)
	}
	if state.Brightness > 100 {
		return fmt.Errorf("brightness must be between 0 and 100")
	}
	return nil
}

func (store *clusterLightingStateStore) Snapshot() (clusterLightingState, error) {
	if store == nil {
		return clusterLightingState{}, fmt.Errorf("RGB Cluster lighting state is unavailable")
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	return store.state, nil
}

func (store *clusterLightingStateStore) Set(state clusterLightingState) error {
	if err := validateClusterLightingState(state); err != nil {
		return err
	}
	if store == nil || store.writer == nil {
		return fmt.Errorf("RGB Cluster lighting state is unavailable")
	}
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("encode RGB Cluster lighting state: %w", err)
	}
	data = append(data, '\n')
	store.mu.Lock()
	defer store.mu.Unlock()
	if err = store.writer.Write(store.path, data); err != nil {
		return fmt.Errorf("persist RGB Cluster lighting state: %w", err)
	}
	store.state = state
	return nil
}

type clusterLayout struct {
	SchemaVersion int      `json:"schemaVersion"`
	DeviceOrder   []string `json:"deviceOrder"`
}

func defaultClusterLayout() clusterLayout {
	return clusterLayout{SchemaVersion: clusterPersistenceSchemaVersion, DeviceOrder: []string{}}
}

type clusterLayoutStore struct {
	mu     sync.RWMutex
	path   string
	writer clusterPersistenceWriter
	layout clusterLayout
}

func loadClusterLayoutStore(path string) (*clusterLayoutStore, error) {
	return loadClusterLayoutStoreWithWriter(path, atomicClusterPersistenceWriter{})
}

func loadClusterLayoutStoreWithWriter(path string, writer clusterPersistenceWriter) (*clusterLayoutStore, error) {
	if strings.TrimSpace(path) == "" {
		return nil, fmt.Errorf("RGB Cluster layout path is empty")
	}
	layout := defaultClusterLayout()
	if err := decodeClusterPersistence(path, &layout); err != nil {
		if !errors.Is(err, os.ErrNotExist) {
			return nil, fmt.Errorf("load RGB Cluster layout %q: %w", path, err)
		}
	} else if err = validateClusterLayout(layout); err != nil {
		return nil, fmt.Errorf("load RGB Cluster layout %q: %w", path, err)
	}
	layout.DeviceOrder = append([]string(nil), layout.DeviceOrder...)
	return &clusterLayoutStore{path: path, writer: writer, layout: layout}, nil
}

func validateClusterLayout(layout clusterLayout) error {
	if layout.SchemaVersion != clusterPersistenceSchemaVersion {
		return fmt.Errorf("unsupported schema version %d", layout.SchemaVersion)
	}
	if layout.DeviceOrder == nil {
		return fmt.Errorf("device order is missing")
	}
	seen := make(map[string]struct{}, len(layout.DeviceOrder))
	for _, serial := range layout.DeviceOrder {
		if strings.TrimSpace(serial) == "" {
			return fmt.Errorf("device order contains an empty serial")
		}
		if _, exists := seen[serial]; exists {
			return fmt.Errorf("device order contains duplicate serial %q", serial)
		}
		seen[serial] = struct{}{}
	}
	return nil
}

func (store *clusterLayoutStore) Snapshot() (clusterLayout, error) {
	if store == nil {
		return clusterLayout{}, fmt.Errorf("RGB Cluster layout is unavailable")
	}
	store.mu.RLock()
	defer store.mu.RUnlock()
	layout := store.layout
	layout.DeviceOrder = append([]string(nil), store.layout.DeviceOrder...)
	return layout, nil
}

func (store *clusterLayoutStore) Set(layout clusterLayout) error {
	layout.DeviceOrder = append([]string(nil), layout.DeviceOrder...)
	if err := validateClusterLayout(layout); err != nil {
		return err
	}
	if store == nil || store.writer == nil {
		return fmt.Errorf("RGB Cluster layout is unavailable")
	}
	data, err := json.MarshalIndent(layout, "", "  ")
	if err != nil {
		return fmt.Errorf("encode RGB Cluster layout: %w", err)
	}
	data = append(data, '\n')
	store.mu.Lock()
	defer store.mu.Unlock()
	if err = store.writer.Write(store.path, data); err != nil {
		return fmt.Errorf("persist RGB Cluster layout: %w", err)
	}
	store.layout = layout
	return nil
}

func decodeClusterPersistence(path string, destination any) error {
	file, err := os.Open(path)
	if err != nil {
		return err
	}
	defer file.Close()
	decoder := json.NewDecoder(io.LimitReader(file, 1<<20))
	decoder.DisallowUnknownFields()
	if err = decoder.Decode(destination); err != nil {
		return err
	}
	if err = decoder.Decode(&struct{}{}); err != io.EOF {
		if err == nil {
			err = fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}
