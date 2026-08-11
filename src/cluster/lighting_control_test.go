package cluster

import (
	"errors"
	"math"
	"os"
	"path/filepath"
	"reflect"
	"testing"

	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func TestClusterLightingSnapshotUsesCanonicalStateAndIsDefensive(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	fresh := device.LightingSnapshot()
	if !fresh.Available || fresh.SelectedEffect != "rainbow" || fresh.Settings.EffectID != "rainbow" || fresh.Customized {
		t.Fatalf("fresh canonical lighting snapshot = %#v", fresh)
	}
	if err := device.SetLightingEffect("gradient"); err != nil {
		t.Fatal(err)
	}
	stops := []lightingsettings.GradientStop{
		{Position: 0, Color: lightingsettings.Color{Red: 1, Green: 2, Blue: 3}, Intensity: 0.25},
		{Position: 1, Color: lightingsettings.Color{Red: 4, Green: 5, Blue: 6}, Intensity: 0.75},
	}
	if err := device.SetLightingGradient("gradient", stops); err != nil {
		t.Fatal(err)
	}
	if err := device.SetLightingBrightness(61); err != nil {
		t.Fatal(err)
	}

	device.mutex.Lock()
	device.DeviceProfile.RGBProfile = "off"
	compatibilityBrightness := uint8(2)
	device.DeviceProfile.BrightnessSlider = &compatibilityBrightness
	device.Rgb.Profiles["gradient"] = rgb.Profile{Speed: 999}
	device.mutex.Unlock()

	snapshot := device.LightingSnapshot()
	if !snapshot.Available || snapshot.SelectedEffect != "gradient" || snapshot.Brightness != 61 ||
		snapshot.EffectiveBrightness != 61 || !snapshot.Customized || snapshot.Settings.EffectID != "gradient" ||
		snapshot.Settings.Gradient == nil || !reflect.DeepEqual(snapshot.Settings.Gradient.Stops, stops) {
		t.Fatalf("canonical lighting snapshot = %#v", snapshot)
	}
	snapshot.Settings.Gradient.Stops[0].Color.Red = 255
	again := device.LightingSnapshot()
	if again.Settings.Gradient == nil || again.Settings.Gradient.Stops[0].Color.Red != 1 {
		t.Fatalf("snapshot settings aliased canonical state: %#v", again.Settings)
	}

	if result := device.SchedulerBrightness(0); result != 1 {
		t.Fatalf("SchedulerBrightness(0) = %d", result)
	}
	lightsOut := device.LightingSnapshot()
	if lightsOut.Brightness != 61 || lightsOut.EffectiveBrightness != 0 {
		t.Fatalf("scheduler snapshot = %#v", lightsOut)
	}
}

func TestClusterLightingSnapshotFailsClosed(t *testing.T) {
	var nilDevice *Device
	if snapshot := nilDevice.LightingSnapshot(); snapshot.Available || snapshot.Settings.EffectID != "" {
		t.Fatalf("nil-device snapshot = %#v", snapshot)
	}
	if snapshot := (&Device{}).LightingSnapshot(); snapshot.Available || snapshot.SelectedEffect != "" || snapshot.Settings.EffectID != "" {
		t.Fatalf("unavailable-state snapshot = %#v", snapshot)
	}

	device, _ := newClusterTestDevice(t)
	device.Stop()
	device.lightingState.mu.Lock()
	device.lightingState.state.SelectedEffect = "missing-effect"
	device.lightingState.mu.Unlock()
	if snapshot := device.LightingSnapshot(); snapshot.Available || snapshot.SelectedEffect != "" || snapshot.Settings.EffectID != "" {
		t.Fatalf("unresolvable-state snapshot = %#v", snapshot)
	}
}

func TestClusterLightingCanonicalMutations(t *testing.T) {
	device, paths := newClusterTestDevice(t)
	for _, brightness := range []uint8{0, 100} {
		if err := device.SetLightingBrightness(brightness); err != nil {
			t.Fatalf("SetLightingBrightness(%d): %v", brightness, err)
		}
		if snapshot := device.LightingSnapshot(); snapshot.Brightness != brightness {
			t.Fatalf("Brightness after %d = %#v", brightness, snapshot)
		}
	}
	if err := device.SetLightingBrightness(63); err != nil {
		t.Fatal(err)
	}
	if err := device.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	waveBefore := device.LightingSnapshot()
	if waveBefore.Settings.TwoColor == nil {
		t.Fatalf("Wave settings = %#v", waveBefore.Settings)
	}
	waveColors := *waveBefore.Settings.TwoColor
	if err := device.SetLightingSpeed("wave", 4); err != nil {
		t.Fatal(err)
	}
	waveAfter := device.LightingSnapshot()
	if waveAfter.Settings.Speed == nil || *waveAfter.Settings.Speed != 4 || waveAfter.Settings.TwoColor == nil ||
		*waveAfter.Settings.TwoColor != waveColors || waveAfter.Brightness != 63 || !waveAfter.Customized {
		t.Fatalf("Wave mutation = %#v", waveAfter)
	}
	if err := device.SetLightingSpeed("rainbow", 4); err == nil {
		t.Fatal("stale expected effect accepted")
	}
	if err := device.SetLightingSpeed("wave", math.NaN()); err == nil {
		t.Fatal("non-finite Speed accepted")
	}

	start := lightingsettings.Color{Red: 10, Green: 20, Blue: 30}
	end := lightingsettings.Color{Red: 40, Green: 50, Blue: 60}
	if err := device.SetLightingTwoColor("wave", start, end); err != nil {
		t.Fatal(err)
	}
	waveAfter = device.LightingSnapshot()
	if waveAfter.Settings.TwoColor == nil || waveAfter.Settings.TwoColor.Start != start || waveAfter.Settings.TwoColor.End != end ||
		waveAfter.Settings.Speed == nil || *waveAfter.Settings.Speed != 4 || waveAfter.Brightness != 63 {
		t.Fatalf("two-color mutation = %#v", waveAfter)
	}

	if err := device.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	single := lightingsettings.Color{Red: 9, Green: 8, Blue: 7}
	if err := device.SetLightingSingleColor("static", single); err != nil {
		t.Fatal(err)
	}
	staticSnapshot := device.LightingSnapshot()
	if staticSnapshot.Settings.SingleColor == nil || staticSnapshot.Settings.SingleColor.Color != single ||
		staticSnapshot.Brightness != 63 || !staticSnapshot.Customized {
		t.Fatalf("single-color mutation = %#v", staticSnapshot)
	}
	if err := device.SetLightingTwoColor("static", start, end); err == nil {
		t.Fatal("wrong-palette two-color mutation accepted")
	}

	if err := device.SetLightingEffect("cpu-temperature"); err != nil {
		t.Fatal(err)
	}
	low := lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Blue: 255}, Celsius: 15}
	middle := lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Green: 255}, Celsius: 47}
	high := lightingsettings.TemperaturePoint{Color: lightingsettings.Color{Red: 255}, Celsius: 91}
	if err := device.SetLightingTemperature("cpu-temperature", low, middle, high); err != nil {
		t.Fatal(err)
	}
	temperature := device.LightingSnapshot()
	if temperature.Settings.Temperature == nil || temperature.Settings.Temperature.Low != low ||
		temperature.Settings.Temperature.Middle != middle || temperature.Settings.Temperature.High != high || temperature.Brightness != 63 {
		t.Fatalf("temperature mutation = %#v", temperature)
	}
	if err := device.SetLightingTemperature("cpu-temperature", low, high, middle); err == nil {
		t.Fatal("unordered temperature thresholds accepted")
	}
	nonFiniteMiddle := middle
	nonFiniteMiddle.Celsius = math.Inf(1)
	if err := device.SetLightingTemperature("cpu-temperature", low, nonFiniteMiddle, high); err == nil {
		t.Fatal("non-finite temperature threshold accepted")
	}

	if err := device.SetLightingEffect("gradient"); err != nil {
		t.Fatal(err)
	}
	gradient := []lightingsettings.GradientStop{
		{Position: 0, Color: lightingsettings.Color{Red: 255}, Intensity: 0.2},
		{Position: 0.5, Color: lightingsettings.Color{Green: 255}, Intensity: 0.4},
		{Position: 0.5, Color: lightingsettings.Color{Blue: 255}, Intensity: 0.6},
		{Position: 1, Color: lightingsettings.Color{Red: 255, Blue: 255}, Intensity: 0.8},
	}
	if err := device.SetLightingGradient("gradient", gradient); err != nil {
		t.Fatal(err)
	}
	gradientSnapshot := device.LightingSnapshot()
	if gradientSnapshot.Settings.Gradient == nil || !reflect.DeepEqual(gradientSnapshot.Settings.Gradient.Stops, gradient) ||
		gradientSnapshot.Brightness != 63 {
		t.Fatalf("Gradient mutation = %#v", gradientSnapshot)
	}
	gradient[0].Color.Red = 1
	if afterCallerMutation := device.LightingSnapshot(); afterCallerMutation.Settings.Gradient.Stops[0].Color.Red != 255 {
		t.Fatalf("Gradient retained caller slice: %#v", afterCallerMutation.Settings.Gradient)
	}
	beforeRejected := device.LightingSnapshot()
	unsorted := []lightingsettings.GradientStop{
		{Position: 1, Color: lightingsettings.Color{Red: 1}, Intensity: 1},
		{Position: 0, Color: lightingsettings.Color{Blue: 1}, Intensity: 1},
	}
	if err := device.SetLightingGradient("gradient", unsorted); err == nil {
		t.Fatal("unordered Gradient accepted")
	}
	nonFinite := append([]lightingsettings.GradientStop(nil), gradient...)
	nonFinite[0].Intensity = math.NaN()
	if err := device.SetLightingGradient("gradient", nonFinite); err == nil {
		t.Fatal("non-finite Gradient intensity accepted")
	}
	tooMany := make([]lightingsettings.GradientStop, 1025)
	for index := range tooMany {
		tooMany[index] = lightingsettings.GradientStop{Position: float64(index) / 1024, Intensity: 1}
	}
	if err := device.SetLightingGradient("gradient", tooMany); err == nil {
		t.Fatal("Gradient above the 1024-stop limit accepted")
	}
	if afterRejected := device.LightingSnapshot(); !reflect.DeepEqual(afterRejected, beforeRejected) {
		t.Fatalf("rejected Gradient changed state: before %#v after %#v", beforeRejected, afterRejected)
	}

	if err := device.SetLightingEffect("off"); err != nil {
		t.Fatal(err)
	}
	if err := device.SetLightingSpeed("off", 1); err == nil {
		t.Fatal("Speed accepted for an effect without Speed")
	}
	if err := device.SetLightingEffect("not-an-effect"); err == nil {
		t.Fatal("unknown effect accepted")
	}
	if err := device.SetLightingBrightness(101); err == nil {
		t.Fatal("brightness above 100 accepted")
	}
	if snapshot := device.LightingSnapshot(); snapshot.SelectedEffect != "off" || snapshot.Brightness != 63 {
		t.Fatalf("rejected mutations changed target state = %#v", snapshot)
	}

	device.Stop()
	reloaded, err := newDevice(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer reloaded.Stop()
	reloadedSnapshot := reloaded.LightingSnapshot()
	if reloadedSnapshot.SelectedEffect != "off" || reloadedSnapshot.Brightness != 63 {
		t.Fatalf("reloaded target state = %#v", reloadedSnapshot)
	}
	reloadedGradient, err := reloaded.resolver.Resolve(lightingsettings.RGBCluster(), "gradient")
	if err != nil || !reloadedGradient.Customized || reloadedGradient.Settings.Gradient == nil ||
		len(reloadedGradient.Settings.Gradient.Stops) != 4 || reloadedGradient.Settings.Gradient.Stops[1].Position != 0.5 ||
		reloadedGradient.Settings.Gradient.Stops[2].Position != 0.5 {
		t.Fatalf("reloaded Gradient = %#v, %v", reloadedGradient, err)
	}
}

func TestClusterLightingSuccessfulMutationReappliesOutput(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	device.AddDeviceController(&common.ClusterController{Serial: "member", LedChannels: 1, WriteColorEx: func([]byte, int) {}})
	before, running := clusterWorkerState(device)
	if !running {
		t.Fatal("test Cluster worker did not start")
	}
	if err := device.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	after, running := clusterWorkerState(device)
	if !running || after != before+1 {
		t.Fatalf("successful effect mutation worker starts = %d -> %d, running %t", before, after, running)
	}
}

func TestClusterLightingResetDeletesOnlySelectedCustomization(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	if err := device.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	staticColor := lightingsettings.Color{Red: 9, Green: 8, Blue: 7}
	if err := device.SetLightingSingleColor("static", staticColor); err != nil {
		t.Fatal(err)
	}
	staticBefore, ok, err := device.effects.Get("static")
	if err != nil || !ok {
		t.Fatalf("stored Static customization = %#v, %t, %v", staticBefore, ok, err)
	}
	if err = device.SetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}
	shippedWave, err := device.resolver.Resolve(lightingsettings.RGBCluster(), "wave")
	if err != nil || shippedWave.Customized {
		t.Fatalf("shipped Wave resolution = %#v, %v", shippedWave, err)
	}
	if err = device.SetLightingSpeed("wave", 4); err != nil {
		t.Fatal(err)
	}
	if err = device.SetLightingBrightness(63); err != nil {
		t.Fatal(err)
	}
	device.AddDeviceController(&common.ClusterController{Serial: "member-one", LedChannels: 1, WriteColorEx: func([]byte, int) {}})
	device.AddDeviceController(&common.ClusterController{Serial: "member-two", LedChannels: 1, WriteColorEx: func([]byte, int) {}})
	controllersBefore, _ := device.controllerSnapshot()
	controllerOrder := func(controllers []*common.ClusterController) []string {
		serials := make([]string, len(controllers))
		for index, controller := range controllers {
			serials[index] = controller.Serial
		}
		return serials
	}
	orderBefore := controllerOrder(controllersBefore)
	layoutBefore, err := device.layout.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	startsBefore, running := clusterWorkerState(device)
	if !running {
		t.Fatal("test Cluster worker did not start")
	}

	if err = device.ResetLightingEffect("wave"); err != nil {
		t.Fatal(err)
	}

	after := device.LightingSnapshot()
	if !after.Available || after.SelectedEffect != "wave" || after.Brightness != 63 || after.Customized ||
		!reflect.DeepEqual(after.Settings, shippedWave.Settings) {
		t.Fatalf("reset Wave snapshot = %#v, shipped %#v", after, shippedWave.Settings)
	}
	if _, customized, err := device.effects.Get("wave"); err != nil || customized {
		t.Fatalf("Wave customization after reset exists=%t err=%v", customized, err)
	}
	staticAfter, customized, err := device.effects.Get("static")
	if err != nil || !customized || !reflect.DeepEqual(staticAfter, staticBefore) {
		t.Fatalf("unrelated Static customization = %#v, %t, %v; want %#v", staticAfter, customized, err, staticBefore)
	}
	controllersAfter, _ := device.controllerSnapshot()
	orderAfter := controllerOrder(controllersAfter)
	layoutAfter, err := device.layout.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(orderAfter, orderBefore) || !reflect.DeepEqual(layoutAfter, layoutBefore) {
		t.Fatalf("reset changed membership/order: controllers %#v -> %#v, layout %#v -> %#v", orderBefore, orderAfter, layoutBefore, layoutAfter)
	}
	if startsAfter, running := clusterWorkerState(device); !running || startsAfter != startsBefore+1 {
		t.Fatalf("successful reset worker starts = %d -> %d, running %t", startsBefore, startsAfter, running)
	}
}

func TestClusterLightingResetMissingCustomizationIsSuccessfulNoOp(t *testing.T) {
	device, paths := newClusterTestDevice(t)
	if err := device.SetLightingBrightness(42); err != nil {
		t.Fatal(err)
	}
	device.AddDeviceController(&common.ClusterController{Serial: "member", LedChannels: 1, WriteColorEx: func([]byte, int) {}})
	before := device.LightingSnapshot()
	startsBefore, _ := clusterWorkerState(device)
	if err := device.ResetLightingEffect("rainbow"); err != nil {
		t.Fatal(err)
	}
	after := device.LightingSnapshot()
	if !reflect.DeepEqual(after, before) || after.Customized || after.SelectedEffect != "rainbow" || after.Brightness != 42 {
		t.Fatalf("no-op reset changed canonical state: before %#v after %#v", before, after)
	}
	if startsAfter, running := clusterWorkerState(device); !running || startsAfter != startsBefore+1 {
		t.Fatalf("no-op reset worker starts = %d -> %d, running %t", startsBefore, startsAfter, running)
	}
	if _, err := os.Stat(paths.ClusterEffectSettingsFile); !errors.Is(err, os.ErrNotExist) {
		t.Fatalf("no-op reset created customization file: %v", err)
	}
}

func TestClusterLightingResetRejectsInvalidStateWithoutMutation(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	if err := device.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	if err := device.SetLightingSingleColor("static", lightingsettings.Color{Red: 1, Green: 2, Blue: 3}); err != nil {
		t.Fatal(err)
	}
	device.AddDeviceController(&common.ClusterController{Serial: "member", LedChannels: 1, WriteColorEx: func([]byte, int) {}})
	before := device.LightingSnapshot()
	startsBefore, _ := clusterWorkerState(device)
	if err := device.ResetLightingEffect("wave"); err == nil {
		t.Fatal("stale expected effect accepted")
	}
	if after := device.LightingSnapshot(); !reflect.DeepEqual(after, before) {
		t.Fatalf("stale reset changed state: before %#v after %#v", before, after)
	}
	if startsAfter, _ := clusterWorkerState(device); startsAfter != startsBefore {
		t.Fatalf("stale reset reapplied output: %d -> %d", startsBefore, startsAfter)
	}

	device.Stop()
	device.lightingState.mu.Lock()
	device.lightingState.state.SelectedEffect = "not-an-effect"
	device.lightingState.mu.Unlock()
	if err := device.ResetLightingEffect("not-an-effect"); err == nil {
		t.Fatal("unknown selected effect accepted")
	}
	stored, customized, err := device.effects.Get("static")
	if err != nil || !customized || stored.SingleColor == nil || stored.SingleColor.Color.Red != 1 {
		t.Fatalf("rejected unknown reset changed customization = %#v, %t, %v", stored, customized, err)
	}

	var nilDevice *Device
	if err := nilDevice.ResetLightingEffect("static"); err == nil {
		t.Fatal("nil runtime reset reported success")
	}
	if err := (&Device{}).ResetLightingEffect("static"); err == nil {
		t.Fatal("unavailable runtime reset reported success")
	}
}

func TestClusterLightingResetPersistenceFailurePreservesCustomizationAndOutput(t *testing.T) {
	device, paths := newClusterTestDevice(t)
	if err := device.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	if err := device.SetLightingSingleColor("static", lightingsettings.Color{Red: 10, Green: 20, Blue: 30}); err != nil {
		t.Fatal(err)
	}
	device.AddDeviceController(&common.ClusterController{Serial: "member", LedChannels: 1, WriteColorEx: func([]byte, int) {}})
	before := device.LightingSnapshot()
	startsBefore, _ := clusterWorkerState(device)
	lightingRoot := filepath.Dir(paths.ClusterEffectSettingsFile)
	movedRoot := lightingRoot + "-moved"
	if err := os.Rename(lightingRoot, movedRoot); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(lightingRoot, []byte("blocks directory recreation"), 0o600); err != nil {
		t.Fatal(err)
	}
	if err := device.ResetLightingEffect("static"); err == nil {
		t.Fatal("reset persistence failure reported success")
	}
	if after := device.LightingSnapshot(); !reflect.DeepEqual(after, before) {
		t.Fatalf("failed reset changed canonical state: before %#v after %#v", before, after)
	}
	if _, customized, err := device.effects.Get("static"); err != nil || !customized {
		t.Fatalf("failed reset removed customization: exists=%t err=%v", customized, err)
	}
	if startsAfter, _ := clusterWorkerState(device); startsAfter != startsBefore {
		t.Fatalf("failed reset reapplied output: %d -> %d", startsBefore, startsAfter)
	}
}

func TestClusterLightingMutationPersistenceFailurePreservesStateAndOutput(t *testing.T) {
	writeErr := errors.New("injected persistence failure")
	t.Run("target state", func(t *testing.T) {
		device, _ := newClusterTestDevice(t)
		device.AddDeviceController(&common.ClusterController{Serial: "member", LedChannels: 1})
		before := device.LightingSnapshot()
		starts, _ := clusterWorkerState(device)
		setClusterLightingStateWriterForTest(device.lightingState, clusterWriterFunc(func(string, []byte) error { return writeErr }))
		if err := device.SetLightingBrightness(25); !errors.Is(err, writeErr) {
			t.Fatalf("SetLightingBrightness error = %v", err)
		}
		if err := device.SetLightingEffect("static"); !errors.Is(err, writeErr) {
			t.Fatalf("SetLightingEffect error = %v", err)
		}
		if after := device.LightingSnapshot(); !reflect.DeepEqual(after, before) {
			t.Fatalf("failed target mutation changed state: before %#v after %#v", before, after)
		}
		if afterStarts, _ := clusterWorkerState(device); afterStarts != starts {
			t.Fatalf("failed target mutation reapplied output: %d -> %d", starts, afterStarts)
		}
	})

	t.Run("effect customization", func(t *testing.T) {
		device, paths := newClusterTestDevice(t)
		if err := device.SetLightingEffect("static"); err != nil {
			t.Fatal(err)
		}
		initial := lightingsettings.Color{Red: 10, Green: 20, Blue: 30}
		if err := device.SetLightingSingleColor("static", initial); err != nil {
			t.Fatal(err)
		}
		device.AddDeviceController(&common.ClusterController{Serial: "member", LedChannels: 1})
		before := device.LightingSnapshot()
		starts, _ := clusterWorkerState(device)
		lightingRoot := filepath.Dir(paths.ClusterEffectSettingsFile)
		movedRoot := lightingRoot + "-moved"
		if err := os.Rename(lightingRoot, movedRoot); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lightingRoot, []byte("blocks directory recreation"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err := device.SetLightingSingleColor("static", lightingsettings.Color{Red: 99}); err == nil {
			t.Fatal("persistence failure reported success")
		}
		if after := device.LightingSnapshot(); !reflect.DeepEqual(after, before) {
			t.Fatalf("failed customization changed state: before %#v after %#v", before, after)
		}
		if afterStarts, _ := clusterWorkerState(device); afterStarts != starts {
			t.Fatalf("failed customization reapplied output: %d -> %d", starts, afterStarts)
		}
	})
}
