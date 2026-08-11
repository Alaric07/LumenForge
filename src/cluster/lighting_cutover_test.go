package cluster

import (
	"bytes"
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"reflect"
	"sync"
	"testing"
	"time"

	"LumenForge/src/common"
	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/rgb"
)

func clusterTestPaths(t *testing.T) config.Paths {
	t.Helper()
	workingDirectory, err := os.Getwd()
	if err != nil {
		t.Fatal(err)
	}
	repositoryRoot := filepath.Clean(filepath.Join(workingDirectory, "..", ".."))
	root := t.TempDir()
	paths, err := config.ResolvePaths(config.PathOptions{
		Mode:             config.ServiceModeDevelopment,
		ApplicationRoot:  repositoryRoot,
		ConfigRoot:       filepath.Join(root, "config"),
		DataRoot:         filepath.Join(root, "data"),
		WorkingDirectory: repositoryRoot,
	})
	if err != nil {
		t.Fatal(err)
	}
	return paths
}

func newClusterTestDevice(t *testing.T) (*Device, config.Paths) {
	t.Helper()
	paths := clusterTestPaths(t)
	device, err := newDevice(paths)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(device.Stop)
	return device, paths
}

func clusterWorkerState(device *Device) (uint64, bool) {
	device.workerMutex.Lock()
	defer device.workerMutex.Unlock()
	return device.workerStarts, device.workerStop != nil
}

func setClusterLightingStateWriterForTest(store *clusterLightingStateStore, writer clusterPersistenceWriter) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.writer = writer
}

func setClusterLayoutWriterForTest(store *clusterLayoutStore, writer clusterPersistenceWriter) {
	store.mu.Lock()
	defer store.mu.Unlock()
	store.writer = writer
}

func TestClusterCleanInstallAndLegacySentinelsAreIgnored(t *testing.T) {
	paths := clusterTestPaths(t)
	legacyRGBPath := filepath.Join(paths.MutableRGBRoot, "cluster.json")
	legacyProfilePath := filepath.Join(paths.MutableProfilesRoot, "cluster.json")
	if err := os.MkdirAll(filepath.Dir(legacyRGBPath), 0o700); err != nil {
		t.Fatal(err)
	}
	if err := os.MkdirAll(filepath.Dir(legacyProfilePath), 0o700); err != nil {
		t.Fatal(err)
	}
	legacyRGB := []byte(`{"profiles":{"off":{"speed":999}}}`)
	legacyProfile := []byte(`{"RGBProfile":"off","BrightnessSlider":1,"DeviceOrder":["legacy"]}`)
	if err := os.WriteFile(legacyRGBPath, legacyRGB, 0o600); err != nil {
		t.Fatal(err)
	}
	if err := os.WriteFile(legacyProfilePath, legacyProfile, 0o600); err != nil {
		t.Fatal(err)
	}

	device, err := newDevice(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer device.Stop()
	snapshot := device.LightingSnapshot()
	if !snapshot.Available || snapshot.SelectedEffect != "rainbow" || snapshot.Brightness != 100 || snapshot.EffectiveBrightness != 100 {
		t.Fatalf("fresh Cluster snapshot = %#v", snapshot)
	}
	for _, path := range []string{
		paths.RGBClusterLightingStateFile,
		paths.RGBClusterLayoutFile,
		paths.ClusterEffectSettingsFile,
	} {
		if _, statErr := os.Stat(path); !errors.Is(statErr, os.ErrNotExist) {
			t.Fatalf("fresh load created %q: %v", path, statErr)
		}
	}
	for path, before := range map[string][]byte{legacyRGBPath: legacyRGB, legacyProfilePath: legacyProfile} {
		after, readErr := os.ReadFile(path)
		if readErr != nil || !bytes.Equal(after, before) {
			t.Fatalf("legacy sentinel %q changed: %q, %v", path, after, readErr)
		}
	}
}

func TestClusterMalformedCanonicalPersistenceFailsClosed(t *testing.T) {
	tests := []struct {
		name string
		path func(config.Paths) string
	}{
		{name: "target state", path: func(paths config.Paths) string { return paths.RGBClusterLightingStateFile }},
		{name: "layout", path: func(paths config.Paths) string { return paths.RGBClusterLayoutFile }},
		{name: "effect settings", path: func(paths config.Paths) string { return paths.ClusterEffectSettingsFile }},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			paths := clusterTestPaths(t)
			path := test.path(paths)
			if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
				t.Fatal(err)
			}
			if err := os.WriteFile(path, []byte("{"), 0o600); err != nil {
				t.Fatal(err)
			}
			device, err := newDevice(paths)
			if err == nil {
				device.Stop()
				t.Fatal("malformed canonical persistence initialized an available Cluster runtime")
			}
			if device.runtimeAvailable() {
				t.Fatal("failed Cluster initialization left canonical mutations available")
			}
			if err = device.SetLightingEffect("static"); err == nil {
				t.Fatal("mutation through unavailable runtime succeeded")
			}
		})
	}
}

func TestClusterTargetStatePersistsAndReloadsCanonically(t *testing.T) {
	paths := clusterTestPaths(t)
	first, err := newDevice(paths)
	if err != nil {
		t.Fatal(err)
	}
	if err = first.SetLightingEffect("static"); err != nil {
		t.Fatalf("SetLightingEffect(static): %v", err)
	}
	if err = first.SetLightingBrightness(42); err != nil {
		t.Fatalf("SetLightingBrightness(42): %v", err)
	}
	first.Stop()

	second, err := newDevice(paths)
	if err != nil {
		t.Fatal(err)
	}
	defer second.Stop()
	snapshot := second.LightingSnapshot()
	if snapshot.SelectedEffect != "static" || snapshot.Brightness != 42 || snapshot.EffectiveBrightness != 42 {
		t.Fatalf("reloaded target state = %#v", snapshot)
	}
	if _, err = os.Stat(paths.RGBClusterLightingStateFile); err != nil {
		t.Fatalf("canonical state file missing: %v", err)
	}
}

func TestClusterResolverCustomizationIsDefensive(t *testing.T) {
	device, paths := newClusterTestDevice(t)
	defaultResolution, err := device.resolver.Resolve(lightingsettings.RGBCluster(), "wave")
	if err != nil || defaultResolution.Customized {
		t.Fatalf("Wave default = %#v, %v", defaultResolution, err)
	}

	if err = device.SetLightingEffect("static"); err != nil {
		t.Fatal(err)
	}
	color := lightingsettings.Color{Red: 12, Green: 34, Blue: 56}
	if err = device.SetLightingSingleColor("static", color); err != nil {
		t.Fatal(err)
	}
	resolution, err := device.resolver.Resolve(lightingsettings.RGBCluster(), "static")
	if err != nil || !resolution.Customized || resolution.Settings.SingleColor == nil || resolution.Settings.SingleColor.Color != color {
		t.Fatalf("Static customization = %#v, %v", resolution, err)
	}
	resolution.Settings.SingleColor.Color.Red = 200
	again, err := device.resolver.Resolve(lightingsettings.RGBCluster(), "static")
	if err != nil || again.Settings.SingleColor == nil || again.Settings.SingleColor.Color != color {
		t.Fatalf("resolved customization aliased canonical state: %#v, %v", again, err)
	}
	if _, err = os.Stat(paths.ClusterEffectSettingsFile); err != nil {
		t.Fatalf("ClusterStore file missing: %v", err)
	}
}

func TestClusterLegacyRGBCompatibilitySurfaceIsAbsent(t *testing.T) {
	deviceType := reflect.TypeOf((*Device)(nil))
	for _, method := range []string{
		"ChangeDeviceBrightnessValue",
		"ControlDeviceRgb",
		"GetRgbProfile",
		"GetRgbProfiles",
		"ProcessDeleteGradientColor",
		"ProcessNewGradientColor",
		"UpdateRgbProfile",
		"UpdateRgbProfileData",
	} {
		if _, ok := deviceType.MethodByName(method); ok {
			t.Errorf("legacy RGB compatibility method %s remains on Cluster", method)
		}
	}
	for _, field := range []string{"DeviceProfile", "Rgb", "RGBModes"} {
		if _, ok := deviceType.Elem().FieldByName(field); ok {
			t.Errorf("legacy RGB compatibility field %s remains on Cluster", field)
		}
	}
}

func TestClusterRendererAdapterPreservesGradientAndTemperatureSettings(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	speed := 10.0
	// Red 202 keeps the renderer's intermediate byte conversion away from an
	// exact quantization boundary while preserving the expected final value.
	gradient := lightingsettings.EffectSettings{
		SchemaVersion: lightingsettings.SchemaVersion,
		EffectID:      "gradient",
		Speed:         &speed,
		Gradient: &lightingsettings.GradientSettings{Stops: []lightingsettings.GradientStop{
			{Position: 0, Color: lightingsettings.Color{Red: 202, Green: 100, Blue: 50}, Intensity: 0.5},
			{Position: 1, Color: lightingsettings.Color{Red: 202, Green: 100, Blue: 50}, Intensity: 0.5},
		}},
	}
	if err := device.effects.Set("gradient", gradient); err != nil {
		t.Fatal(err)
	}
	profile := rgbProfileFromSettings(gradient)
	if len(profile.Gradients) != 2 || profile.Gradients[0].Position != 0 || profile.Gradients[1].Position != 1 || profile.Gradients[0].Brightness != 0.5 {
		t.Fatalf("Gradient renderer profile = %#v", profile)
	}
	startTime := time.Now()
	frame := device.generateRgbEffectFromProfile(2, &startTime, "gradient", profile, 50, rgb.Exit())
	// The renderer's established per-stop intensity result is then scaled once
	// by the owning 50% Cluster Brightness.
	want := []byte{25, 12, 6, 25, 12, 6}
	if !bytes.Equal(frame, want) {
		t.Fatalf("Gradient frame = %v, want %v", frame, want)
	}

	resolution, err := device.resolver.Resolve(lightingsettings.RGBCluster(), "cpu-temperature")
	if err != nil {
		t.Fatal(err)
	}
	temperature := rgbProfileFromSettings(resolution.Settings)
	if temperature.StartColor.Temperature != 20 || temperature.MiddleColor.Temperature != 50 || temperature.EndColor.Temperature != 95 {
		t.Fatalf("CPU temperature renderer profile = %#v", temperature)
	}
	if err = device.SetLightingEffect("cpu-temperature"); err != nil {
		t.Fatal(err)
	}
	low := lightingsettings.TemperaturePoint{Celsius: 10, Color: lightingsettings.Color{Red: 1}}
	middle := lightingsettings.TemperaturePoint{Celsius: 77, Color: lightingsettings.Color{Green: 2}}
	high := lightingsettings.TemperaturePoint{Celsius: 90, Color: lightingsettings.Color{Blue: 3}}
	if err = device.SetLightingTemperature("cpu-temperature", low, middle, high); err != nil {
		t.Fatal(err)
	}
	resolution, err = device.resolver.Resolve(lightingsettings.RGBCluster(), "cpu-temperature")
	if err != nil {
		t.Fatal(err)
	}
	updated := rgbProfileFromSettings(resolution.Settings)
	if updated.StartColor.Temperature != 10 || updated.MiddleColor.Temperature != 77 || updated.EndColor.Temperature != 90 || updated.MiddleColor.Green != 2 {
		t.Fatalf("updated temperature renderer profile = %#v", updated)
	}
}

func TestClusterNilActiveRGBFailsClosed(t *testing.T) {
	device := &Device{}
	startTime := time.Unix(0, 0)
	frame := device.generateRgbEffectFromProfile(2, &startTime, "rainbow", rgb.Profile{}, 100, nil)
	if !bytes.Equal(frame, make([]byte, 6)) {
		t.Fatalf("nil ActiveRGB frame = %v", frame)
	}
}

func TestClusterLayoutPersistsOrdersSegmentsAndMigratesSerials(t *testing.T) {
	device, paths := newClusterTestDevice(t)
	if result := device.UpdateDeviceOrder([]string{"second", "first"}); result != 1 {
		t.Fatalf("UpdateDeviceOrder = %d", result)
	}
	var first, second []byte
	device.mutex.Lock()
	device.Controllers = []*common.ClusterController{
		{Serial: "first", LedChannels: 1, WriteColorEx: func(data []byte, _ int) { first = append([]byte(nil), data...) }},
		{Serial: "second", LedChannels: 1, WriteColorEx: func(data []byte, _ int) { second = append([]byte(nil), data...) }},
	}
	device.mutex.Unlock()
	device.SortControllers()
	controllers, _ := device.controllerSnapshot()
	if len(controllers) != 2 || controllers[0].Serial != "second" || controllers[1].Serial != "first" {
		t.Fatalf("ordered controllers = %#v", controllers)
	}
	distributeColorsToControllers([]byte{1, 2, 3, 4, 5, 6}, controllers)
	if !bytes.Equal(second, []byte{1, 2, 3}) || !bytes.Equal(first, []byte{4, 5, 6}) {
		t.Fatalf("ordered segments = second %v first %v", second, first)
	}

	reloaded, err := loadClusterLayoutStore(paths.RGBClusterLayoutFile)
	if err != nil {
		t.Fatal(err)
	}
	layout, _ := reloaded.Snapshot()
	if !reflect.DeepEqual(layout.DeviceOrder, []string{"second", "first"}) {
		t.Fatalf("reloaded order = %v", layout.DeviceOrder)
	}
	device.MigrateDeviceOrderSerial("second", "renamed")
	layout, _ = device.layout.Snapshot()
	if !reflect.DeepEqual(layout.DeviceOrder, []string{"renamed", "first"}) {
		t.Fatalf("migrated order = %v", layout.DeviceOrder)
	}
	data, err := os.ReadFile(paths.RGBClusterLayoutFile)
	if err != nil {
		t.Fatal(err)
	}
	var fields map[string]json.RawMessage
	if err = json.Unmarshal(data, &fields); err != nil {
		t.Fatal(err)
	}
	if len(fields) != 2 || fields["schemaVersion"] == nil || fields["deviceOrder"] == nil {
		t.Fatalf("layout persisted non-layout state: %s", data)
	}
}

func TestClusterUpdateDeviceOrderRejectsDuplicatesWithoutChangingCanonicalLayout(t *testing.T) {
	device, paths := newClusterTestDevice(t)
	want := []string{"first", "second"}
	if result := device.UpdateDeviceOrder(want); result != 1 {
		t.Fatalf("valid UpdateDeviceOrder = %d", result)
	}
	device.AddDeviceController(&common.ClusterController{Serial: "first", LedChannels: 1})
	starts, _ := clusterWorkerState(device)
	before, err := device.layout.Snapshot()
	if err != nil {
		t.Fatal(err)
	}
	dataBefore, err := os.ReadFile(paths.RGBClusterLayoutFile)
	if err != nil {
		t.Fatal(err)
	}

	if result := device.UpdateDeviceOrder([]string{"first", "first"}); result != 0 {
		t.Fatalf("duplicate UpdateDeviceOrder = %d", result)
	}
	after, err := device.layout.Snapshot()
	if err != nil || !reflect.DeepEqual(after, before) {
		t.Fatalf("layout after duplicate rejection = %#v, want %#v, %v", after, before, err)
	}
	dataAfter, err := os.ReadFile(paths.RGBClusterLayoutFile)
	if err != nil || !bytes.Equal(dataAfter, dataBefore) {
		t.Fatalf("persisted layout changed after duplicate rejection: %q, %v", dataAfter, err)
	}
	if afterStarts, _ := clusterWorkerState(device); afterStarts != starts {
		t.Fatalf("duplicate layout rejection restarted worker: %d -> %d", starts, afterStarts)
	}
	reloaded, err := loadClusterLayoutStore(paths.RGBClusterLayoutFile)
	if err != nil {
		t.Fatalf("reload after duplicate rejection: %v", err)
	}
	persisted, err := reloaded.Snapshot()
	if err != nil || !reflect.DeepEqual(persisted.DeviceOrder, want) {
		t.Fatalf("reloaded layout = %#v, want %v, %v", persisted, want, err)
	}
}

func TestClusterSortControllersOrdersRankedMembersBeforeUnrankedAndNil(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	if result := device.UpdateDeviceOrder([]string{"ranked-first", "ranked-second"}); result != 1 {
		t.Fatalf("UpdateDeviceOrder = %d", result)
	}
	unrankedFirst := &common.ClusterController{Serial: "unranked-first"}
	rankedSecond := &common.ClusterController{Serial: "ranked-second"}
	rankedFirst := &common.ClusterController{Serial: "ranked-first"}
	unrankedSecond := &common.ClusterController{Serial: "unranked-second"}
	device.mutex.Lock()
	device.Controllers = []*common.ClusterController{nil, unrankedFirst, rankedSecond, nil, rankedFirst, unrankedSecond}
	device.mutex.Unlock()

	device.SortControllers()

	device.mutex.RLock()
	controllers := append([]*common.ClusterController(nil), device.Controllers...)
	device.mutex.RUnlock()
	want := []*common.ClusterController{rankedFirst, rankedSecond, unrankedFirst, unrankedSecond, nil, nil}
	if !reflect.DeepEqual(controllers, want) {
		t.Fatalf("sorted controllers = %#v, want %#v", controllers, want)
	}
}

func TestClusterLayoutSerialMigrationDeduplicatesCollision(t *testing.T) {
	device, paths := newClusterTestDevice(t)
	if result := device.UpdateDeviceOrder([]string{"old", "middle", "new"}); result != 1 {
		t.Fatalf("UpdateDeviceOrder = %d", result)
	}
	device.MigrateDeviceOrderSerial("old", "new")
	want := []string{"new", "middle"}
	layout, err := device.layout.Snapshot()
	if err != nil || !reflect.DeepEqual(layout.DeviceOrder, want) {
		t.Fatalf("migrated collision order = %v, %v", layout.DeviceOrder, err)
	}
	reloaded, err := loadClusterLayoutStore(paths.RGBClusterLayoutFile)
	if err != nil {
		t.Fatal(err)
	}
	persisted, err := reloaded.Snapshot()
	if err != nil || !reflect.DeepEqual(persisted.DeviceOrder, want) {
		t.Fatalf("persisted collision order = %v, %v", persisted.DeviceOrder, err)
	}
}

func TestClusterWorkerLifecycleHasOneDeterministicOwner(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	frames := make(chan struct{}, 16)
	callback := func([]byte, int) {
		select {
		case frames <- struct{}{}:
		default:
		}
	}
	device.AddDeviceController(&common.ClusterController{Serial: "first", LedChannels: 1, WriteColorEx: callback})
	select {
	case <-frames:
	case <-time.After(time.Second):
		t.Fatal("first Cluster worker produced no frame")
	}
	starts, running := clusterWorkerState(device)
	if starts != 1 || !running {
		t.Fatalf("first member worker = starts %d running %t", starts, running)
	}
	device.AddDeviceController(&common.ClusterController{Serial: "second", LedChannels: 1, WriteColorEx: callback})
	if starts, _ = clusterWorkerState(device); starts != 1 {
		t.Fatalf("second member created duplicate worker: %d", starts)
	}
	if err := device.SetLightingEffect("static"); err != nil {
		t.Fatalf("effect mutation: %v", err)
	}
	if starts, _ = clusterWorkerState(device); starts != 2 {
		t.Fatalf("effect mutation worker starts = %d", starts)
	}
	if err := device.SetLightingSingleColor("static", lightingsettings.Color{Red: 20, Green: 30, Blue: 40}); err != nil {
		t.Fatalf("customization mutation: %v", err)
	}
	if starts, _ = clusterWorkerState(device); starts != 3 {
		t.Fatalf("customization worker starts = %d", starts)
	}
	if result := device.UpdateDeviceOrder([]string{"second", "first"}); result != 1 {
		t.Fatalf("layout mutation = %d", result)
	}
	if starts, _ = clusterWorkerState(device); starts != 4 {
		t.Fatalf("layout worker starts = %d", starts)
	}
	device.RemoveDeviceControllerBySerial("first")
	if _, running = clusterWorkerState(device); !running {
		t.Fatal("removing a non-last member stopped the worker")
	}
	device.RemoveDeviceControllerBySerial("second")
	if starts, running = clusterWorkerState(device); starts != 4 || running {
		t.Fatalf("last removal worker = starts %d running %t", starts, running)
	}
	device.AddDeviceController(&common.ClusterController{Serial: "third", LedChannels: 1, WriteColorEx: callback})
	if starts, running = clusterWorkerState(device); starts != 5 || !running {
		t.Fatalf("reconnect worker = starts %d running %t", starts, running)
	}
}

func TestClusterStopIsSafeWhenRepeatedAndConcurrent(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	const callers = 8
	var wait sync.WaitGroup
	wait.Add(callers)
	for index := 0; index < callers; index++ {
		go func() {
			defer wait.Done()
			device.Stop()
		}()
	}
	done := make(chan struct{})
	go func() {
		wait.Wait()
		close(done)
	}()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("concurrent Stop calls did not return")
	}
	device.Stop()
}

func TestClusterPendingRestartCoalescesAndUsesLatestCanonicalState(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	callbackEntered := make(chan struct{})
	releaseCallback := make(chan struct{})
	replacementFrames := make(chan []byte, 1)
	var callbackMutex sync.Mutex
	callbackCalls := 0
	device.AddDeviceController(&common.ClusterController{
		Serial:      "blocked",
		LedChannels: 1,
		WriteColorEx: func(data []byte, _ int) {
			callbackMutex.Lock()
			callbackCalls++
			call := callbackCalls
			callbackMutex.Unlock()
			if call == 1 {
				close(callbackEntered)
				<-releaseCallback
				return
			}
			select {
			case replacementFrames <- append([]byte(nil), data...):
			default:
			}
		},
	})
	select {
	case <-callbackEntered:
	case <-time.After(time.Second):
		t.Fatal("Cluster callback was not invoked")
	}

	device.workerMutex.Lock()
	blockedDone := device.workerDone
	device.workerMutex.Unlock()
	mutationDone := make(chan error, 1)
	go func() { mutationDone <- device.SetLightingEffect("static") }()
	select {
	case err := <-mutationDone:
		if err != nil {
			t.Fatalf("selected-effect mutation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("pending restart request did not return boundedly")
	}

	if err := device.SetLightingBrightness(50); err != nil {
		t.Fatalf("Brightness mutation: %v", err)
	}
	if err := device.SetLightingSingleColor("static", lightingsettings.Color{Red: 200, Green: 100, Blue: 50}); err != nil {
		t.Fatalf("Static customization: %v", err)
	}
	device.mutex.Lock()
	device.Controllers[0].LedChannels = 2
	device.mutex.Unlock()
	device.workerMutex.Lock()
	if device.workerStarts != 1 || device.workerDone != blockedDone || !device.workerStopping || !device.workerRestartPending {
		t.Fatalf("coalesced restart state = starts %d stopping %t pending %t done %p, want one pending owner %p", device.workerStarts, device.workerStopping, device.workerRestartPending, device.workerDone, blockedDone)
	}
	device.workerMutex.Unlock()

	close(releaseCallback)
	select {
	case frame := <-replacementFrames:
		want := []byte{100, 50, 25, 100, 50, 25}
		if !bytes.Equal(frame, want) {
			t.Fatalf("replacement frame = %v, want %v", frame, want)
		}
	case <-time.After(time.Second):
		t.Fatal("pending Cluster restart produced no replacement frame")
	}
	device.workerMutex.Lock()
	if device.workerStarts != 2 || device.workerDone == nil || device.workerDone == blockedDone || device.workerStopping || device.workerRestartPending {
		t.Fatalf("replacement ownership = starts %d stopping %t pending %t done %p", device.workerStarts, device.workerStopping, device.workerRestartPending, device.workerDone)
	}
	device.workerMutex.Unlock()
}

func TestClusterBlockedCallbackKeepsWorkerOwnershipWithoutBlockingStop(t *testing.T) {
	device, _ := newClusterTestDevice(t)
	callbackEntered := make(chan struct{})
	releaseCallback := make(chan struct{})
	var enterOnce sync.Once
	device.AddDeviceController(&common.ClusterController{
		Serial:      "blocked",
		LedChannels: 1,
		WriteColorEx: func([]byte, int) {
			enterOnce.Do(func() { close(callbackEntered) })
			<-releaseCallback
		},
	})
	select {
	case <-callbackEntered:
	case <-time.After(time.Second):
		t.Fatal("Cluster callback was not invoked")
	}

	device.workerMutex.Lock()
	blockedDone := device.workerDone
	device.workerMutex.Unlock()
	mutationDone := make(chan error, 1)
	go func() { mutationDone <- device.SetLightingEffect("static") }()
	select {
	case err := <-mutationDone:
		if err != nil {
			t.Fatalf("selected-effect mutation: %v", err)
		}
	case <-time.After(time.Second):
		t.Fatal("worker replacement waited indefinitely")
	}
	device.workerMutex.Lock()
	if device.workerStarts != 1 || device.workerDone != blockedDone || !device.workerStopping || !device.workerRestartPending {
		t.Fatalf("blocked worker ownership = starts %d stopping %t pending %t done %p, want one pending owner %p", device.workerStarts, device.workerStopping, device.workerRestartPending, device.workerDone, blockedDone)
	}
	device.workerMutex.Unlock()

	stopDone := make(chan struct{})
	go func() {
		device.Stop()
		close(stopDone)
	}()
	select {
	case <-stopDone:
	case <-time.After(time.Second):
		t.Fatal("Stop waited indefinitely for blocked callback")
	}
	device.workerMutex.Lock()
	if device.workerStarts != 1 || device.workerDone != blockedDone || device.workerRestartPending {
		t.Fatalf("Stop ownership = starts %d pending %t done %p, want stopped pending state with owner %p", device.workerStarts, device.workerRestartPending, device.workerDone, blockedDone)
	}
	device.workerMutex.Unlock()

	close(releaseCallback)
	select {
	case <-blockedDone:
	case <-time.After(time.Second):
		t.Fatal("released Cluster worker did not exit")
	}
	ticker := time.NewTicker(time.Millisecond)
	defer ticker.Stop()
	deadline := time.NewTimer(time.Second)
	defer deadline.Stop()
	for {
		device.workerMutex.Lock()
		clean := device.workerDone == nil && device.workerStop == nil && device.activeRgb == nil && !device.workerStopping && !device.workerRestartPending
		starts := device.workerStarts
		device.workerMutex.Unlock()
		if clean {
			if starts != 1 {
				t.Fatalf("blocked worker cleanup created %d workers", starts)
			}
			break
		}
		select {
		case <-ticker.C:
		case <-deadline.C:
			t.Fatal("exited Cluster worker retained ownership")
		}
	}
}

func TestClusterMutationFailuresPreserveStateLayoutCustomizationAndWorker(t *testing.T) {
	writeErr := errors.New("injected persistence failure")
	t.Run("state", func(t *testing.T) {
		device, _ := newClusterTestDevice(t)
		device.AddDeviceController(&common.ClusterController{Serial: "member", LedChannels: 1})
		before := device.LightingSnapshot()
		starts, _ := clusterWorkerState(device)
		setClusterLightingStateWriterForTest(device.lightingState, clusterWriterFunc(func(string, []byte) error { return writeErr }))
		if err := device.SetLightingBrightness(25); err == nil {
			t.Fatal("failed Brightness mutation succeeded")
		}
		if err := device.SetLightingEffect("static"); err == nil {
			t.Fatal("failed selected-effect mutation succeeded")
		}
		if after := device.LightingSnapshot(); !reflect.DeepEqual(after, before) {
			t.Fatalf("failed state mutation changed memory: before %#v after %#v", before, after)
		}
		if afterStarts, _ := clusterWorkerState(device); afterStarts != starts {
			t.Fatalf("failed state mutation replaced worker: %d -> %d", starts, afterStarts)
		}
	})

	t.Run("layout", func(t *testing.T) {
		device, _ := newClusterTestDevice(t)
		device.AddDeviceController(&common.ClusterController{Serial: "first", LedChannels: 1})
		device.AddDeviceController(&common.ClusterController{Serial: "second", LedChannels: 1})
		controllersBefore, _ := device.controllerSnapshot()
		before, _ := device.layout.Snapshot()
		starts, _ := clusterWorkerState(device)
		setClusterLayoutWriterForTest(device.layout, clusterWriterFunc(func(string, []byte) error { return writeErr }))
		if result := device.UpdateDeviceOrder([]string{"second", "first"}); result != 0 {
			t.Fatalf("failed layout mutation = %d", result)
		}
		after, _ := device.layout.Snapshot()
		if !reflect.DeepEqual(after, before) {
			t.Fatalf("failed layout mutation changed memory: before %#v after %#v", before, after)
		}
		if afterStarts, _ := clusterWorkerState(device); afterStarts != starts {
			t.Fatalf("failed layout mutation replaced worker: %d -> %d", starts, afterStarts)
		}
		controllersAfter, _ := device.controllerSnapshot()
		if len(controllersBefore) != 2 || len(controllersAfter) != 2 || controllersBefore[0].Serial != controllersAfter[0].Serial || controllersBefore[1].Serial != controllersAfter[1].Serial {
			t.Fatalf("failed layout mutation reordered controllers: before %#v after %#v", controllersBefore, controllersAfter)
		}
	})

	t.Run("customization", func(t *testing.T) {
		device, paths := newClusterTestDevice(t)
		if err := device.SetLightingEffect("static"); err != nil {
			t.Fatalf("unable to select Static: %v", err)
		}
		initial := lightingsettings.Color{Red: 10}
		if err := device.SetLightingSingleColor("static", initial); err != nil {
			t.Fatalf("unable to seed Static customization: %v", err)
		}
		device.AddDeviceController(&common.ClusterController{Serial: "member", LedChannels: 1})
		starts, _ := clusterWorkerState(device)
		before, err := device.resolver.Resolve(lightingsettings.RGBCluster(), "static")
		if err != nil || before.Settings.SingleColor == nil {
			t.Fatalf("Static customization is unavailable before persistence failure: %#v, %v", before, err)
		}
		lightingRoot := filepath.Dir(paths.ClusterEffectSettingsFile)
		movedRoot := lightingRoot + "-moved"
		if err := os.Rename(lightingRoot, movedRoot); err != nil {
			t.Fatal(err)
		}
		if err := os.WriteFile(lightingRoot, []byte("blocks directory recreation"), 0o600); err != nil {
			t.Fatal(err)
		}
		if err = device.SetLightingSingleColor("static", lightingsettings.Color{Red: 99}); err == nil {
			t.Fatal("failed customization mutation succeeded")
		}
		after, err := device.resolver.Resolve(lightingsettings.RGBCluster(), "static")
		if err != nil || after.Settings.SingleColor == nil || after.Settings.SingleColor.Color != before.Settings.SingleColor.Color {
			t.Fatalf("failed customization changed resolved state: before %#v after %#v, %v", before, after, err)
		}
		if afterStarts, _ := clusterWorkerState(device); afterStarts != starts {
			t.Fatalf("failed customization replaced worker: %d -> %d", starts, afterStarts)
		}
	})
}

func TestClusterSchedulerAndOffUseOnlyCanonicalTargetState(t *testing.T) {
	device, paths := newClusterTestDevice(t)
	if device.SetLightingBrightness(65) != nil || device.SchedulerBrightness(0) != 1 {
		t.Fatal("unable to enable scheduler lights-out")
	}
	snapshot := device.LightingSnapshot()
	if snapshot.Brightness != 65 || snapshot.EffectiveBrightness != 0 {
		t.Fatalf("scheduler lights-out snapshot = %#v", snapshot)
	}
	if device.SetLightingBrightness(40) != nil {
		t.Fatal("unable to change canonical Brightness while lights-out")
	}
	snapshot = device.LightingSnapshot()
	if snapshot.Brightness != 40 || snapshot.EffectiveBrightness != 0 {
		t.Fatalf("lights-out canonical change = %#v", snapshot)
	}
	if device.SchedulerBrightness(1) != 1 {
		t.Fatal("unable to clear scheduler lights-out")
	}
	snapshot = device.LightingSnapshot()
	if snapshot.Brightness != 40 || snapshot.EffectiveBrightness != 40 {
		t.Fatalf("scheduler restore snapshot = %#v", snapshot)
	}
	reloaded, err := loadClusterLightingStateStore(paths.RGBClusterLightingStateFile)
	if err != nil {
		t.Fatal(err)
	}
	persisted, _ := reloaded.Snapshot()
	if persisted.Brightness != 40 {
		t.Fatalf("canonical Brightness was not persisted: %#v", persisted)
	}

	if err = device.SetLightingEffect("static"); err != nil {
		t.Fatalf("unable to select non-Rainbow effect: %v", err)
	}
	if device.LightingSnapshot().SelectedEffect != "static" {
		t.Fatal("non-Rainbow effect selection did not persist Static")
	}
	if err = device.SetLightingEffect("off"); err != nil {
		t.Fatalf("unable to select Off: %v", err)
	}
	if device.LightingSnapshot().SelectedEffect != "off" {
		t.Fatal("canonical effect selection did not persist Off")
	}
	startTime := time.Now()
	frame := device.generateRgbEffect(2, &startTime, device.LightingSnapshot().SelectedEffect, rgb.Exit())
	if !bytes.Equal(frame, make([]byte, 6)) {
		t.Fatalf("canonical Off output = %v", frame)
	}
	if device.SchedulerBrightness(1) != 1 || device.LightingSnapshot().SelectedEffect != "off" {
		t.Fatal("scheduler Brightness changed canonical Off selection")
	}
}
