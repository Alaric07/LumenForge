package deviceprofilepresentation

import "testing"

type completeProfileMutationDevice struct{}

func (completeProfileMutationDevice) ChangeDeviceProfile(string) uint8 { return 1 }
func (completeProfileMutationDevice) SaveUserProfile(string) uint8     { return 1 }
func (completeProfileMutationDevice) DeleteDeviceProfile(string) uint8 { return 1 }

type noDeleteProfileMutationDevice struct{}

func (noDeleteProfileMutationDevice) ChangeDeviceProfile(string) uint8 { return 1 }
func (noDeleteProfileMutationDevice) SaveUserProfile(string) uint8     { return 1 }

func TestWithMutationCapabilitiesExposesOnlyCompleteProfileActions(t *testing.T) {
	complete := WithMutationCapabilities(Snapshot{Supported: true}, completeProfileMutationDevice{})
	if !complete.CanSwitch || !complete.CanSave || !complete.CanDelete {
		t.Fatalf("complete capabilities = %#v", complete)
	}

	noDelete := WithMutationCapabilities(Snapshot{Supported: true}, noDeleteProfileMutationDevice{})
	if !noDelete.CanSwitch || !noDelete.CanSave || noDelete.CanDelete {
		t.Fatalf("no-delete capabilities = %#v", noDelete)
	}
}
