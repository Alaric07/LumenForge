package lightingsettings

import (
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
)

func testResolver(t *testing.T) (*Resolver, *DeviceStore, *ClusterStore) {
	t.Helper()
	root := t.TempDir()
	devices, err := LoadDeviceStore(filepath.Join(root, "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := LoadClusterStore(filepath.Join(root, "cluster.json"))
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewResolver(loadTestDefaults(t), devices, cluster)
	if err != nil {
		t.Fatal(err)
	}
	return resolver, devices, cluster
}

func TestTargetSpecificResolverConstructionAndResolution(t *testing.T) {
	root := t.TempDir()
	defaults := loadTestDefaults(t)
	devices, err := LoadDeviceStore(filepath.Join(root, "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := LoadClusterStore(filepath.Join(root, "cluster.json"))
	if err != nil {
		t.Fatal(err)
	}

	deviceResolver, err := NewDeviceResolver(defaults, devices)
	if err != nil {
		t.Fatal(err)
	}
	clusterResolver, err := NewClusterResolver(defaults, cluster)
	if err != nil {
		t.Fatal(err)
	}

	deviceDefault, err := deviceResolver.Resolve(IndependentDevice("device-default"), "static")
	if err != nil || deviceDefault.Customized || deviceDefault.Settings.SingleColor == nil {
		t.Fatalf("device default = %#v, %v", deviceDefault, err)
	}
	clusterDefault, err := clusterResolver.Resolve(RGBCluster(), "static")
	if err != nil || clusterDefault.Customized || !reflect.DeepEqual(clusterDefault.Settings, deviceDefault.Settings) {
		t.Fatalf("cluster default = %#v, %v", clusterDefault, err)
	}
	deviceDefault.Settings.SingleColor.Color.Red = 77
	deviceDefaultAgain, err := deviceResolver.Resolve(IndependentDevice("device-default"), "static")
	if err != nil || deviceDefaultAgain.Settings.SingleColor.Color.Red == 77 {
		t.Fatalf("device default aliased repository = %#v, %v", deviceDefaultAgain, err)
	}

	deviceCustom := testStaticSettings(11)
	if err = devices.Set("device-custom", "static", deviceCustom); err != nil {
		t.Fatal(err)
	}
	resolvedDevice, err := deviceResolver.Resolve(IndependentDevice("device-custom"), "static")
	if err != nil || !resolvedDevice.Customized || !reflect.DeepEqual(resolvedDevice.Settings, deviceCustom) {
		t.Fatalf("device custom = %#v, %v", resolvedDevice, err)
	}
	resolvedDevice.Settings.SingleColor.Color.Red = 99
	resolvedDeviceAgain, err := deviceResolver.Resolve(IndependentDevice("device-custom"), "static")
	if err != nil || resolvedDeviceAgain.Settings.SingleColor.Color.Red != 11 {
		t.Fatalf("device custom aliased store = %#v, %v", resolvedDeviceAgain, err)
	}

	clusterCustom := testStaticSettings(22)
	if err = cluster.Set("static", clusterCustom); err != nil {
		t.Fatal(err)
	}
	resolvedCluster, err := clusterResolver.Resolve(RGBCluster(), "static")
	if err != nil || !resolvedCluster.Customized || !reflect.DeepEqual(resolvedCluster.Settings, clusterCustom) {
		t.Fatalf("cluster custom = %#v, %v", resolvedCluster, err)
	}
	resolvedCluster.Settings.SingleColor.Color.Red = 88
	resolvedClusterAgain, err := clusterResolver.Resolve(RGBCluster(), "static")
	if err != nil || resolvedClusterAgain.Settings.SingleColor.Color.Red != 22 {
		t.Fatalf("cluster custom aliased store = %#v, %v", resolvedClusterAgain, err)
	}

	const clusterDependencyError = "cluster lighting settings store is unavailable"
	for attempt := 0; attempt < 2; attempt++ {
		if _, resolveErr := deviceResolver.Resolve(RGBCluster(), "static"); resolveErr == nil || resolveErr.Error() != clusterDependencyError {
			t.Fatalf("device resolver cluster error = %v, want %q", resolveErr, clusterDependencyError)
		}
	}
	const deviceDependencyError = "device lighting settings store is unavailable"
	for attempt := 0; attempt < 2; attempt++ {
		if _, resolveErr := clusterResolver.Resolve(IndependentDevice("device"), "static"); resolveErr == nil || resolveErr.Error() != deviceDependencyError {
			t.Fatalf("cluster resolver device error = %v, want %q", resolveErr, deviceDependencyError)
		}
	}
	if _, resolveErr := deviceResolver.Resolve(Target{Kind: TargetRGBCluster, ID: "other"}, "static"); !errors.Is(resolveErr, ErrInvalidTarget) {
		t.Fatalf("device resolver invalid cluster identity error = %v", resolveErr)
	}
	if _, resolveErr := clusterResolver.Resolve(IndependentDevice(""), "static"); !errors.Is(resolveErr, ErrInvalidTarget) {
		t.Fatalf("cluster resolver invalid device identity error = %v", resolveErr)
	}
	if _, resolveErr := deviceResolver.Resolve(IndependentDevice("device"), "unknown"); !errors.Is(resolveErr, ErrUnknownEffect) {
		t.Fatalf("device resolver invalid effect error = %v", resolveErr)
	}
}

func TestTargetSpecificResolverConstructorsRejectMissingDependencies(t *testing.T) {
	root := t.TempDir()
	defaults := loadTestDefaults(t)
	devices, err := LoadDeviceStore(filepath.Join(root, "devices.json"))
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := LoadClusterStore(filepath.Join(root, "cluster.json"))
	if err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name      string
		construct func() error
	}{
		{name: "device defaults", construct: func() error { _, err := NewDeviceResolver(nil, devices); return err }},
		{name: "device store", construct: func() error { _, err := NewDeviceResolver(defaults, nil); return err }},
		{name: "cluster defaults", construct: func() error { _, err := NewClusterResolver(nil, cluster); return err }},
		{name: "cluster store", construct: func() error { _, err := NewClusterResolver(defaults, nil); return err }},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if err := test.construct(); err == nil {
				t.Fatal("constructor accepted a missing dependency")
			}
		})
	}
}

func TestResolverDefaultCustomizationIsolationAndReset(t *testing.T) {
	resolver, devices, cluster := testResolver(t)
	deviceOne := IndependentDevice("device-one")
	deviceTwo := IndependentDevice("device-two")

	deviceDefault, err := resolver.Resolve(deviceOne, "static")
	if err != nil || deviceDefault.Customized {
		t.Fatalf("device default = %#v, %v", deviceDefault, err)
	}
	clusterDefault, err := resolver.Resolve(RGBCluster(), "static")
	if err != nil || clusterDefault.Customized || !reflect.DeepEqual(deviceDefault.Settings, clusterDefault.Settings) {
		t.Fatalf("cluster default = %#v, %v", clusterDefault, err)
	}

	deviceCustom := testStaticSettings(11)
	clusterCustom := testStaticSettings(22)
	if err = devices.Set(deviceOne.ID, "static", deviceCustom); err != nil {
		t.Fatal(err)
	}
	resolved, err := resolver.Resolve(deviceOne, "static")
	if err != nil || !resolved.Customized || !reflect.DeepEqual(resolved.Settings, deviceCustom) {
		t.Fatalf("device custom = %#v, %v", resolved, err)
	}
	if resolvedTwo, resolveErr := resolver.Resolve(deviceTwo, "static"); resolveErr != nil || resolvedTwo.Customized || reflect.DeepEqual(resolvedTwo.Settings, deviceCustom) {
		t.Fatalf("second device was affected = %#v, %v", resolvedTwo, resolveErr)
	}
	if stillDefault, resolveErr := resolver.Resolve(RGBCluster(), "static"); resolveErr != nil || stillDefault.Customized {
		t.Fatalf("device custom affected cluster = %#v, %v", stillDefault, resolveErr)
	}
	if otherEffect, resolveErr := resolver.Resolve(deviceOne, "wave"); resolveErr != nil || otherEffect.Customized {
		t.Fatalf("device Static custom affected Wave = %#v, %v", otherEffect, resolveErr)
	}

	if err = cluster.Set("static", clusterCustom); err != nil {
		t.Fatal(err)
	}
	resolvedCluster, err := resolver.Resolve(RGBCluster(), "static")
	if err != nil || !resolvedCluster.Customized || !reflect.DeepEqual(resolvedCluster.Settings, clusterCustom) {
		t.Fatalf("cluster custom = %#v, %v", resolvedCluster, err)
	}
	resolvedDevice, err := resolver.Resolve(deviceOne, "static")
	if err != nil || !resolvedDevice.Customized || !reflect.DeepEqual(resolvedDevice.Settings, deviceCustom) {
		t.Fatalf("cluster custom affected device = %#v, %v", resolvedDevice, err)
	}

	resolvedDevice.Settings.SingleColor.Color.Red = 99
	again, err := resolver.Resolve(deviceOne, "static")
	if err != nil || again.Settings.SingleColor.Color.Red != 11 {
		t.Fatalf("resolved custom aliased store = %#v, %v", again, err)
	}
	deviceDefault.Settings.SingleColor.Color.Red = 88
	defaultAgain, err := resolver.Resolve(deviceTwo, "static")
	if err != nil || defaultAgain.Settings.SingleColor.Color.Red == 88 {
		t.Fatalf("resolved default aliased repository = %#v, %v", defaultAgain, err)
	}

	deleted, err := devices.Delete(deviceOne.ID, "static")
	if err != nil || !deleted {
		t.Fatalf("Delete(device custom) = %t, %v", deleted, err)
	}
	reset, err := resolver.Resolve(deviceOne, "static")
	if err != nil || reset.Customized || !reflect.DeepEqual(reset.Settings, defaultAgain.Settings) {
		t.Fatalf("device reset did not restore hidden default = %#v, %v", reset, err)
	}
	deleted, err = cluster.Delete("static")
	if err != nil || !deleted {
		t.Fatalf("Delete(cluster custom) = %t, %v", deleted, err)
	}
	clusterReset, err := resolver.Resolve(RGBCluster(), "static")
	if err != nil || clusterReset.Customized || !reflect.DeepEqual(clusterReset.Settings, reset.Settings) {
		t.Fatalf("cluster reset did not restore hidden default = %#v, %v", clusterReset, err)
	}
}

func TestResolverPerformsNoWrites(t *testing.T) {
	root := t.TempDir()
	devicePath := filepath.Join(root, "devices.json")
	clusterPath := filepath.Join(root, "cluster.json")
	devices, err := LoadDeviceStore(devicePath)
	if err != nil {
		t.Fatal(err)
	}
	cluster, err := LoadClusterStore(clusterPath)
	if err != nil {
		t.Fatal(err)
	}
	resolver, err := NewResolver(loadTestDefaults(t), devices, cluster)
	if err != nil {
		t.Fatal(err)
	}
	for _, target := range []Target{IndependentDevice("device"), RGBCluster()} {
		if _, err = resolver.Resolve(target, "gradient"); err != nil {
			t.Fatal(err)
		}
	}
	for _, path := range []string{devicePath, clusterPath} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("Resolve created %q: %v", path, statErr)
		}
	}

	if err = devices.Set("device", "static", testStaticSettings(7)); err != nil {
		t.Fatal(err)
	}
	before, err := os.ReadFile(devicePath)
	if err != nil {
		t.Fatal(err)
	}
	for index := 0; index < 10; index++ {
		if _, err = resolver.Resolve(IndependentDevice("device"), "static"); err != nil {
			t.Fatal(err)
		}
	}
	after, err := os.ReadFile(devicePath)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before, after) {
		t.Fatal("Resolve rewrote customization persistence")
	}
}

func TestResolverRejectsInvalidTargetsEffectsAndUnavailableDefaults(t *testing.T) {
	resolver, _, _ := testResolver(t)
	tests := []struct {
		name   string
		target Target
		effect string
		want   error
	}{
		{name: "unknown kind", target: Target{Kind: 99, ID: "device"}, effect: "static", want: ErrInvalidTarget},
		{name: "empty device", target: IndependentDevice(""), effect: "static", want: ErrInvalidTarget},
		{name: "wrong cluster identity", target: Target{Kind: TargetRGBCluster, ID: "other"}, effect: "static", want: ErrInvalidTarget},
		{name: "unknown effect", target: IndependentDevice("device"), effect: "unknown", want: ErrUnknownEffect},
	}
	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			if _, err := resolver.Resolve(test.target, test.effect); !errors.Is(err, test.want) {
				t.Fatalf("Resolve() error = %v, want %v", err, test.want)
			}
		})
	}
	delete(resolver.defaults.settings, "static")
	if _, err := resolver.Resolve(IndependentDevice("device"), "static"); !errors.Is(err, ErrDefaultUnavailable) {
		t.Fatalf("unavailable default error = %v", err)
	}
}

func TestResolverConcurrentReadsReturnIndependentData(t *testing.T) {
	resolver, devices, _ := testResolver(t)
	if err := devices.Set("custom", "gradient", testGradientSettings(5)); err != nil {
		t.Fatal(err)
	}

	var wait sync.WaitGroup
	errorsChannel := make(chan error, 64)
	for worker := 0; worker < 32; worker++ {
		for _, target := range []Target{IndependentDevice("default"), IndependentDevice("custom")} {
			wait.Add(1)
			go func(target Target) {
				defer wait.Done()
				for iteration := 0; iteration < 100; iteration++ {
					resolution, err := resolver.Resolve(target, "gradient")
					if err != nil {
						errorsChannel <- err
						return
					}
					*resolution.Settings.Speed = 1
					resolution.Settings.Gradient.Stops[0].Color.Red = float64(iteration)
				}
			}(target)
		}
	}
	wait.Wait()
	close(errorsChannel)
	for err := range errorsChannel {
		t.Error(err)
	}
	defaultResult, err := resolver.Resolve(IndependentDevice("default"), "gradient")
	if err != nil || *defaultResult.Settings.Speed != 10 || defaultResult.Settings.Gradient.Stops[0].Color.Red != 255 {
		t.Fatalf("concurrent default reads mutated repository: %#v, %v", defaultResult, err)
	}
	customResult, err := resolver.Resolve(IndependentDevice("custom"), "gradient")
	if err != nil || *customResult.Settings.Speed != 5 || customResult.Settings.Gradient.Stops[0].Color.Red != 255 {
		t.Fatalf("concurrent custom reads mutated store: %#v, %v", customResult, err)
	}
}
