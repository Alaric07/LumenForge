// Package deviceprofilepresentation defines full device-profile data for the
// shared Devices overview workspace.
package deviceprofilepresentation

// Snapshot is a read-only full device-profile capability snapshot.
type Snapshot struct {
	Supported                  bool
	CanSwitch                  bool
	CanSave                    bool
	CanDelete                  bool
	Profiles                   []string
	ActiveProfile              string
	Scope                      string
	DefaultProfileDisplayLabel string
}

// WithMutationCapabilities records the complete legacy profile mutations a
// concrete provider can safely expose. The shared legacy dispatcher invokes
// these methods with one string argument and expects a uint8 result, so any
// missing or incompatible method is deliberately treated as unsupported.
func WithMutationCapabilities(snapshot Snapshot, device interface{}) Snapshot {
	_, snapshot.CanSwitch = device.(interface{ ChangeDeviceProfile(string) uint8 })
	_, snapshot.CanSave = device.(interface{ SaveUserProfile(string) uint8 })
	_, snapshot.CanDelete = device.(interface{ DeleteDeviceProfile(string) uint8 })
	return snapshot
}

const (
	ScopeDevice               = "device"
	ScopeLighting             = "lighting"
	WorkingConfigurationLabel = "Working Configuration"
)
