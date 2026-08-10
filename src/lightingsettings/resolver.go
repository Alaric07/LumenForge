package lightingsettings

import "fmt"

// TargetKind identifies one of the two active software-lighting ownership scopes.
type TargetKind uint8

const (
	TargetIndependentDevice TargetKind = iota + 1
	TargetRGBCluster

	// RGBClusterIdentity is the established singleton cluster serial.
	RGBClusterIdentity = "cluster"
)

// Target explicitly pairs a supported target kind with its stable identity.
type Target struct {
	Kind TargetKind
	ID   string
}

// IndependentDevice returns an explicitly typed independent-device target.
func IndependentDevice(deviceID string) Target {
	return Target{Kind: TargetIndependentDevice, ID: deviceID}
}

// RGBCluster returns the established singleton RGB Cluster target.
func RGBCluster() Target {
	return Target{Kind: TargetRGBCluster, ID: RGBClusterIdentity}
}

// Resolution contains one complete validated effect definition and whether it
// came from target customization rather than the hidden shipped default.
type Resolution struct {
	Settings   EffectSettings
	Customized bool
}

// Resolver is the canonical read path for independent-device and RGB Cluster
// effect settings.
type Resolver struct {
	defaults *DefaultRepository
	devices  *DeviceStore
	cluster  *ClusterStore
}

// NewResolver constructs the canonical target/effect resolver.
func NewResolver(defaults *DefaultRepository, devices *DeviceStore, cluster *ClusterStore) (*Resolver, error) {
	if defaults == nil || devices == nil || cluster == nil {
		return nil, fmt.Errorf("lighting settings resolver dependencies are incomplete")
	}
	return &Resolver{defaults: defaults, devices: devices, cluster: cluster}, nil
}

// NewDeviceResolver constructs a resolver for independent-device settings.
func NewDeviceResolver(defaults *DefaultRepository, devices *DeviceStore) (*Resolver, error) {
	if defaults == nil || devices == nil {
		return nil, fmt.Errorf("device lighting settings resolver dependencies are incomplete")
	}
	return &Resolver{defaults: defaults, devices: devices}, nil
}

// NewClusterResolver constructs a resolver for RGB Cluster settings.
func NewClusterResolver(defaults *DefaultRepository, cluster *ClusterStore) (*Resolver, error) {
	if defaults == nil || cluster == nil {
		return nil, fmt.Errorf("cluster lighting settings resolver dependencies are incomplete")
	}
	return &Resolver{defaults: defaults, cluster: cluster}, nil
}

// Resolve returns a defensive complete definition without mutating persistence.
func (resolver *Resolver) Resolve(target Target, effectID string) (Resolution, error) {
	if resolver == nil || resolver.defaults == nil {
		return Resolution{}, fmt.Errorf("lighting settings resolver is unavailable")
	}
	if err := validateEffectIdentity(effectID); err != nil {
		return Resolution{}, err
	}

	var (
		settings EffectSettings
		found    bool
		err      error
	)
	switch target.Kind {
	case TargetIndependentDevice:
		if err = validateDeviceIdentity(target.ID); err != nil {
			return Resolution{}, err
		}
		if resolver.devices == nil {
			return Resolution{}, fmt.Errorf("device lighting settings store is unavailable")
		}
		settings, found, err = resolver.devices.Get(target.ID, effectID)
	case TargetRGBCluster:
		if target.ID != RGBClusterIdentity {
			return Resolution{}, fmt.Errorf("%w: RGB Cluster identity must be %q", ErrInvalidTarget, RGBClusterIdentity)
		}
		if resolver.cluster == nil {
			return Resolution{}, fmt.Errorf("cluster lighting settings store is unavailable")
		}
		settings, found, err = resolver.cluster.Get(effectID)
	default:
		return Resolution{}, fmt.Errorf("%w: unsupported target kind %d", ErrInvalidTarget, target.Kind)
	}
	if err != nil {
		return Resolution{}, err
	}
	if found {
		return Resolution{Settings: settings, Customized: true}, nil
	}
	settings, err = resolver.defaults.Get(effectID)
	if err != nil {
		return Resolution{}, err
	}
	return Resolution{Settings: settings}, nil
}
