package m65rgbultraWU

import (
	"LumenForge/src/cluster"
	"LumenForge/src/common"
	"LumenForge/src/lightingsettings"
	"LumenForge/src/logger"
	"LumenForge/src/openrgb"
	"LumenForge/src/rgb"
	"encoding/binary"
	"fmt"
	"sort"
	"strconv"
	"sync"
	"time"
)

type lightingOwner uint8

const (
	lightingNative lightingOwner = iota
	lightingCluster
	lightingOpenRGB
)

type lightingSession struct {
	kind lightingOwner
	stop chan struct{}
}

func newLightingSession(clusterOwner, openRGBOwner bool) *lightingSession {
	s := &lightingSession{stop: make(chan struct{})}
	if clusterOwner {
		s.kind = lightingCluster
	}
	if openRGBOwner {
		s.kind = lightingOpenRGB
	}
	return s
}

type lightingWorker struct{ stop, done chan struct{} }

type lightingApplyOutcome uint8

const (
	lightingStale lightingApplyOutcome = iota
	lightingApplied
	lightingRetry
)
const lightingOneShotAttempts = 3

type lightingOutput struct {
	logo     [3]byte
	session  *lightingSession
	worker   *lightingWorker
	sequence uint64
	replay   bool
	blank    bool // External one-shot Logo blank; never a replay of cached color.
	done     chan lightingApplyOutcome
}
type lightingFrameState struct {
	effect     string
	brightness uint8
	blank      bool
	palette    map[string]lightingsettings.Color
	profile    *rgb.Profile
	cpu, gpu   float32
}

// Cancellation belongs to the private worker, not rgb.ActiveRGB.Exit. That RGB
// object is used only for the algorithms' random-color interpolation state.
func (d *Device) cancelNativeLocked() {
	if w := d.lightingWorker; w != nil {
		close(w.stop)
		d.lightingWorker = nil
	}
}
func lightingDelay(stop <-chan struct{}, duration time.Duration) bool {
	timer := time.NewTimer(duration)
	defer timer.Stop()
	select {
	case <-stop:
		return false
	case <-timer.C:
		return true
	}
}

// WU never rebinds a receiver/endpoint. A terminal lifetime plus an owner token
// and worker identity are enough. Input preferences are copied on their existing
// replay calls; asynchronous Lighting output never dereferences profile maps.
func (d *Device) captureIndicatorLocked() {
	d.lightingIndicatorOK = false
	if d.DeviceProfile == nil {
		return
	}
	// Validate only the effective indicator, as the legacy WU renderer does:
	// an inactive regular stage's missing color cannot suppress valid Sniper RGB.
	var color *rgb.Color
	if d.SniperMode {
		for _, p := range d.DeviceProfile.Profiles {
			if p.Sniper {
				color = p.Color
				break
			}
		}
	} else {
		color = d.DeviceProfile.Profiles[d.DeviceProfile.Profile].Color
	}
	if color == nil || !m65rgbultraWUColorOK(*color) {
		return
	}
	d.lightingIndicator = *color
	d.lightingIndicatorOK = true
}

func (d *Device) lightingFrameLocked() (lightingFrameState, error) {
	var frame lightingFrameState
	if d.lightingCanonical == nil || d.lightingStopped {
		return frame, fmt.Errorf("Lighting unavailable")
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		return frame, err
	}
	frame.effect, frame.brightness = state.SelectedEffect, state.Brightness
	frame.blank = d.userRGBOff || d.schedulerDark
	// Logo blackout depends on the current session/indicator, never on a valid
	// selected palette. Keep ordinary effect resolution strict when unblanking.
	if frame.blank {
		return frame, nil
	}
	// WU has no sampler; retain its existing fields without adding telemetry.
	frame.cpu, frame.gpu = d.CpuTemp, d.GpuTemp
	if frame.effect == "mouse" {
		frame.palette, err = d.lightingCanonical.palette(d.Serial)
	} else {
		var resolved lightingsettings.Resolution
		resolved, err = d.resolveLightingSettings(frame.effect)
		if err == nil {
			// Rendering uses dense positional indices in canonical stop order.
			// Authored keys belong only to storage and the compatibility editor.
			p := lightingsettings.RendererProfileFromEffectSettings(resolved.Settings)
			frame.profile = &p
		}
	}
	return frame, err
}

// Only three authored bytes cross this boundary. Fixed hardware positions are
// not read from legacy profiles, including their serialized ColorIndex fields.
func composeLightingPacket(logo [3]byte, indicator rgb.Color, brightness uint8) []byte {
	factor := float64(brightness) / 100
	return []byte{logo[0], byte(indicator.Red * factor), logo[1], byte(indicator.Green * factor), logo[2], byte(indicator.Blue * factor)}
}

func (d *Device) startCanonicalOutputLocked() {
	if d.lightingStarted {
		return
	}
	d.lightingStarted = true
	go d.lightingOutputLoop()
}

// Publish replacement work in the same critical section as the state change.
// A delayed caller must not retire a newer worker after releasing mutation locks.
func (d *Device) refreshCanonicalLightingLocked(blankExternal bool) {
	if d.lightingStopped || d.lightingCanonical == nil || !d.lightingHardwareReady || d.lightingProfileChanging || d.lightingRecovering {
		return
	}
	d.startCanonicalOutputLocked()
	d.cancelNativeLocked()
	session := d.owner
	if session.kind != lightingNative {
		// RGB-off is one-shot Logo blanking. Ordinary DPI/brightness refresh replays
		// the last successfully supplied Logo, never locally recolors external data.
		blankExternal = blankExternal || d.lightingBlankPending
		d.lightingBlankPending = false
		req := &lightingOutput{session: session, replay: !blankExternal, blank: blankExternal}
		d.enqueueLightingLocked(req)
		return
	}
	frame, err := d.lightingFrameLocked()
	if err != nil {
		return
	}
	worker := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
	d.lightingWorker = worker // publish before the goroutine can render
	go func() {
		defer close(worker.done)
		owner := rgb.Exit()
		owner.RGBStartColor, owner.RGBEndColor = rgb.GenerateRandomColor(1), rgb.GenerateRandomColor(1)
		start := time.Now()
		oneShot := frame.blank || frame.effect == "mouse" || frame.effect == "static"
		failures := 0
		for {
			select {
			case <-worker.stop:
				return
			default:
			}
			data := renderLightingFrame(frame, owner, &start)
			if len(data) != 3 {
				return
			}
			req := &lightingOutput{logo: [3]byte{data[0], data[1], data[2]}, session: session, worker: worker, done: make(chan lightingApplyOutcome, 1)}
			d.lightingMu.Lock()
			valid := !d.lightingStopped && d.owner == session && d.lightingWorker == worker
			if valid {
				d.enqueueLightingLocked(req)
			}
			d.lightingMu.Unlock()
			if !valid {
				return
			}
			// A blocked legacy HID Read cannot hold up renderer cancellation. The one
			// output pump owns transport; there is never a goroutine per USB write.
			var outcome lightingApplyOutcome
			select {
			case <-worker.stop:
				return
			case outcome = <-req.done:
			}
			if outcome == lightingStale {
				return
			}
			if oneShot {
				if outcome == lightingApplied {
					return
				}
				failures++
				if failures >= lightingOneShotAttempts {
					return
				}
			}
			if !lightingDelay(worker.stop, 40*time.Millisecond) {
				return
			}
		}
	}()
}

func finishLightingOutput(req *lightingOutput, outcome lightingApplyOutcome) {
	if req != nil && req.done != nil {
		req.done <- outcome
		close(req.done)
	}
}

// One mailbox coalesces pending output. OpenRGB producers never wait for HID;
// dropping the previous pending request preserves the latest supplied state.
func (d *Device) enqueueLightingLocked(req *lightingOutput) {
	if req.replay {
		if len(d.lightingQueue) > 0 {
			return
		}
	} else {
		d.lightingSequence++
	}
	req.sequence = d.lightingSequence
	select {
	case old := <-d.lightingQueue:
		finishLightingOutput(old, lightingStale)
	default:
	}
	d.lightingQueue <- req
}

func (d *Device) enqueueExternal(data []byte, session *lightingSession) {
	if len(data) != 3 {
		return
	}
	d.lightingMu.Lock()
	defer d.lightingMu.Unlock()
	if d.lightingStopped || d.lightingCanonical == nil || d.owner != session || session.kind == lightingNative || !d.lightingHardwareReady {
		return
	}
	d.startCanonicalOutputLocked()
	d.enqueueLightingLocked(&lightingOutput{logo: [3]byte{data[0], data[1], data[2]}, session: session})
}

func (d *Device) lightingOutputLoop() {
	defer close(d.lightingDone)
	for {
		select {
		case <-d.lightingStop:
			return
		case req := <-d.lightingQueue:
			outcome := lightingStale
			for attempt := 1; attempt <= lightingOneShotAttempts; attempt++ {
				outcome = d.applyLightingOutput(req)
				if (!req.blank && !req.replay) || outcome != lightingRetry || attempt == lightingOneShotAttempts {
					break
				}
				// A blank/replay retry retains its original sequence and owner. New
				// external frames supersede it at the existing final check; replays
				// preserve the current external Logo, and cancellation interrupts pacing.
				if !lightingDelay(req.session.stop, 20*time.Millisecond) {
					outcome = lightingStale
					break
				}
			}
			finishLightingOutput(req, outcome)
			if req.session.kind == lightingOpenRGB && !lightingDelay(req.session.stop, 20*time.Millisecond) {
				// Owner retirement ends this pacing interval, not the object's output pump.
				select {
				case <-d.lightingStop:
					return
				default:
				}
			}
		}
	}
}

// Cancellable lock acquisition keeps a renderer/owner retired while an older
// command is stalled. Existing Write/Read framing and serialization are intact.
func lockLightingOutput(mu *sync.Mutex, stop, ownerStop <-chan struct{}) bool {
	for {
		select {
		case <-stop:
			return false
		case <-ownerStop:
			return false
		default:
		}
		if mu.TryLock() {
			return true
		}
		timer := time.NewTimer(time.Millisecond)
		select {
		case <-stop:
			timer.Stop()
			return false
		case <-ownerStop:
			timer.Stop()
			return false
		case <-timer.C:
		}
	}
}

// Caller holds lightingMu; use the same authorization after an I/O error so a
// retired session is cancelled rather than classified as a retryable failure.
func (d *Device) lightingOutputCurrentLocked(req *lightingOutput) bool {
	valid := !d.lightingStopped && !d.lightingProfileChanging && d.lightingHardwareReady && d.owner == req.session && req.sequence == d.lightingSequence
	if req.worker != nil {
		valid = valid && d.lightingWorker == req.worker && req.session.kind == lightingNative
	}
	return valid
}

func (d *Device) applyLightingOutput(req *lightingOutput) lightingApplyOutcome {
	if d.lightingBeforeApply != nil {
		d.lightingBeforeApply()
	}
	if !lockLightingOutput(&d.deviceLock, d.lightingStop, req.session.stop) {
		return lightingStale
	}
	defer d.deviceLock.Unlock()
	if !lockLightingOutput(&d.mutex, d.lightingStop, req.session.stop) {
		return lightingStale
	}
	defer d.mutex.Unlock()
	// This is the final output boundary, AFTER both USB serialization locks.
	d.lightingMu.Lock()
	if !d.lightingOutputCurrentLocked(req) {
		d.lightingMu.Unlock()
		return lightingStale
	}
	if !d.lightingIndicatorOK {
		d.lightingMu.Unlock()
		return lightingRetry
	}
	logo := req.logo
	if req.replay {
		if !d.lightingLastLogoOK {
			d.lightingMu.Unlock()
			return lightingStale
		}
		logo = d.lightingLastLogo
	}
	state, _, err := d.lightingCanonical.state.Resolve(d.Serial)
	if err != nil {
		d.lightingMu.Unlock()
		return lightingRetry
	}
	brightness := state.Brightness
	if d.schedulerDark {
		brightness = 0
	}
	packet := composeLightingPacket(logo, d.lightingIndicator, brightness)
	d.lightingMu.Unlock()
	buffer := make([]byte, 4+len(packet))
	binary.LittleEndian.PutUint16(buffer[:2], uint16(len(packet)))
	copy(buffer[4:], packet)
	_, err = d.transferLocked(cmdWriteColor, buffer, "canonical Lighting")
	if err != nil {
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to write to color endpoint")
		d.lightingMu.Lock()
		current := d.lightingOutputCurrentLocked(req)
		d.lightingMu.Unlock()
		if !current {
			return lightingStale
		}
		return lightingRetry
	}
	d.lightingMu.Lock()
	if !d.lightingStopped && d.owner == req.session && req.session.kind != lightingNative {
		d.lightingLastLogo, d.lightingLastLogoOK = logo, true
	}
	d.lightingMu.Unlock()
	return lightingApplied
}

// Called only for terminal USB-object retirement, never a receiver disconnect.
// Stop returns without joining the legacy blocking HID Read. Native rendering
// and queued callbacks retire immediately; an already issued HID call cannot be
// interrupted without changing the audited transport contract.
func (d *Device) stopCanonicalLighting() {
	d.lightingMu.Lock()
	if d.lightingStopped {
		d.lightingMu.Unlock()
		return
	}
	d.lightingStopped = true
	d.cancelNativeLocked()
	if d.owner != nil {
		close(d.owner.stop)
	}
	if d.lightingStop != nil {
		close(d.lightingStop)
	}
	d.lightingLastLogoOK = false
	select {
	case req := <-d.lightingQueue:
		finishLightingOutput(req, lightingStale)
	default:
	}
	d.lightingMu.Unlock()
	d.refreshLightingRegistrations()
}

func (d *Device) refreshLightingRegistrations() {
	d.lightingMutation.Lock()
	defer d.lightingMutation.Unlock()
	d.refreshLightingRegistrationsLocked()
}
func (d *Device) refreshLightingRegistrationsLocked() {
	d.lightingMu.Lock()
	available := d.lightingCanonical != nil && !d.lightingStopped
	ready := available && d.lightingHardwareReady
	session := d.owner
	d.lightingMu.Unlock()
	if d.lightingClusterRemove != nil {
		d.lightingClusterRemove(d.Serial)
	} else {
		cluster.Get().RemoveDeviceControllerBySerial(d.Serial)
	}
	if ready && session.kind == lightingCluster {
		c := &common.ClusterController{Product: d.Product, Serial: d.Serial, LedChannels: 1, WriteColorEx: func(data []byte, _ int) { d.enqueueExternal(data, session) }}
		if d.lightingClusterRegister != nil {
			d.lightingClusterRegister(c)
		} else {
			cluster.Get().AddDeviceController(c)
		}
	}
	var c *common.OpenRGBController
	// Reconcile(non-nil) replaces in place. Once registered, keep a gated
	// controller while this USB object is temporarily not ready, including an
	// exhausted recovery attempt. Removing it here would shift other SDK IDs.
	// enqueueExternal still rejects all writes until readiness returns. Initial
	// setup failure has no slot to retain; terminal retirement always removes it.
	if ready || (available && d.lightingOpenRGBRegistered) {
		c = &common.OpenRGBController{Name: d.Product, Vendor: "Corsair", Description: "LumenForge Backend Device", FwVersion: d.Firmware, Serial: d.Serial, Location: fmt.Sprintf("HID: %s", d.Path), Colors: make([]byte, 3), DeviceType: common.DeviceTypeMouse, ColorMode: common.ColorModePerLed, Zones: []common.OpenRGBZone{{Name: "Logo", NumLEDs: 1, ZoneType: common.ZoneTypeLinear}}, WriteColorEx: func(data []byte, _ int) {
			if session.kind == lightingOpenRGB {
				d.enqueueExternal(data, session)
			}
		}}
	}
	if d.lightingOpenRGBReconcile != nil {
		d.lightingOpenRGBReconcile(d.Serial, c)
	} else {
		openrgb.ReconcileDeviceController(d.Serial, c)
	}
	d.lightingOpenRGBRegistered = c != nil
}

func (d *Device) setLightingOwnership(clusterOwner, enabled bool) uint8 {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.lightingStopped || d.DeviceProfile == nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	p := cloneLightingProfile(*d.DeviceProfile)
	if (clusterOwner && p.OpenRGBIntegration) || (!clusterOwner && p.RGBCluster) {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 2
	}
	current := d.owner.kind == lightingOpenRGB
	preference := p.OpenRGBIntegration
	if clusterOwner {
		current = d.owner.kind == lightingCluster
		preference = p.RGBCluster
	}
	if current == enabled && preference == enabled {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 1
	}
	if clusterOwner {
		p.RGBCluster = enabled
	} else {
		p.OpenRGBIntegration = enabled
	}
	if err := d.persistLightingPreference(p); err != nil {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	d.DeviceProfile.RGBCluster, d.DeviceProfile.OpenRGBIntegration = p.RGBCluster, p.OpenRGBIntegration
	d.retireLightingOwnerLocked(p.RGBCluster, p.OpenRGBIntegration)
	d.refreshCanonicalLightingLocked(!clusterOwner && enabled)
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked()
	d.lightingMutation.Unlock()
	return 1
}
func (d *Device) retireLightingOwnerLocked(clusterOwner, openRGBOwner bool) {
	d.cancelNativeLocked()
	if d.owner != nil {
		close(d.owner.stop)
	}
	d.owner = newLightingSession(clusterOwner, openRGBOwner)
	d.lightingLastLogoOK = false
	d.lightingBlankPending = d.userRGBOff || openRGBOwner
}

// Retain the original profile/input operations and their error tolerance. Only
// Lighting ownership retirement/replay is new; no input transaction revisions.
func (d *Device) changeLightingDeviceProfile(name string) uint8 {
	d.lightingMutation.Lock()
	profile, ok := d.UserProfiles[name]
	if !ok || profile == nil || (profile.RGBCluster && profile.OpenRGBIntegration) {
		d.lightingMutation.Unlock()
		return 0
	}
	d.lightingMu.Lock()
	if d.lightingStopped {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	d.lightingProfileChanging = true
	d.retireLightingOwnerLocked(profile.RGBCluster, profile.OpenRGBIntegration)
	d.lightingMu.Unlock()
	d.DeviceProfile.Active = false
	d.saveDeviceProfile()
	profile.Active = true
	d.DeviceProfile = profile
	d.saveDeviceProfile()
	d.lightingMu.Lock()
	d.lightingProfileChanging = false
	if d.lightingStopped {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return 0
	}
	// Retain the loader's effective profile even when its existing void saves
	// tolerate failure; do not publish a different Lighting owner from that profile.
	effectiveOwner := lightingNative
	if d.DeviceProfile.RGBCluster {
		effectiveOwner = lightingCluster
	}
	if d.DeviceProfile.OpenRGBIntegration {
		effectiveOwner = lightingOpenRGB
	}
	if d.owner.kind != effectiveOwner {
		d.retireLightingOwnerLocked(d.DeviceProfile.RGBCluster, d.DeviceProfile.OpenRGBIntegration)
	}
	d.userRGBOff = d.DeviceProfile.RgbOff
	d.lightingBlankPending = d.userRGBOff || d.owner.kind == lightingOpenRGB
	d.captureIndicatorLocked()
	d.lightingMu.Unlock()
	d.refreshLightingRegistrationsLocked()
	d.lightingMutation.Unlock()
	// toggleDPI performs the one required Lighting replay after X/Y programming.
	d.toggleDPI()
	d.loadKeyAssignments()
	d.setupKeyAssignment()
	return 1
}
func renderLightingFrame(value lightingFrameState, owner *rgb.ActiveRGB, start *time.Time) []byte {
	frame := make([]byte, 3)
	if value.blank {
		return frame
	}
	if value.effect == "mouse" || value.effect == "static" {
		factor := float64(value.brightness) / 100
		for id := 0; id < 1; id++ {
			c := value.palette[strconv.Itoa(id)]
			if value.effect == "static" {
				if value.profile == nil {
					return nil
				}
				c = lightingColor(value.profile.StartColor)
			}
			frame[id*3], frame[id*3+1], frame[id*3+2] = byte(c.Red*factor), byte(c.Green*factor), byte(c.Blue*factor)
		}
		return frame
	}
	if value.profile == nil {
		return nil
	}
	profile := copyLightingRGBProfile(*value.profile)
	speed := common.FClamp(profile.Speed, 0.1, 10)
	custom := profile.StartColor != (rgb.Color{}) && profile.EndColor != (rgb.Color{})
	r := rgb.New(2, speed, nil, nil, 1, common.Clamp(profile.Smoothness, 1, 100), time.Duration(speed)*time.Second, custom)
	if custom {
		r.RGBStartColor, r.RGBMiddleColor, r.RGBEndColor = &profile.StartColor, &profile.MiddleColor, &profile.EndColor
	} else {
		r.RGBStartColor, r.RGBMiddleColor, r.RGBEndColor = owner.RGBStartColor, owner.RGBMiddleColor, owner.RGBEndColor
	}
	if r.RGBMiddleColor == nil {
		r.RGBMiddleColor = &rgb.Color{}
	}
	r.RGBBrightness = float64(value.brightness) / 100
	r.RGBStartColor.Brightness, r.RGBEndColor.Brightness, r.RGBMiddleColor.Brightness = r.RGBBrightness, r.RGBBrightness, r.RGBBrightness
	switch value.effect {
	case "off":
		return make([]byte, 3)
	case "rainbow":
		r.Rainbow(*start)
	case "pastelrainbow":
		r.PastelRainbow(*start)
	case "watercolor":
		r.Watercolor(*start)
	case "gradient":
		dense := map[int]rgb.Color{}
		keys := make([]int, 0, len(profile.Gradients))
		for k := range profile.Gradients {
			keys = append(keys, k)
		}
		sort.Ints(keys)
		sort.SliceStable(keys, func(i, j int) bool { return profile.Gradients[keys[i]].Position < profile.Gradients[keys[j]].Position })
		for i, k := range keys {
			dense[i] = profile.Gradients[k]
		}
		r.ColorshiftGradient(*start, dense, profile.Speed)
	case "cpu-temperature":
		r.MinTemp, r.MaxTemp = profile.MinTemp, profile.MaxTemp
		r.Temperature(float64(value.cpu))
	case "gpu-temperature":
		r.MinTemp, r.MaxTemp = profile.MinTemp, profile.MaxTemp
		r.Temperature(float64(value.gpu))
	case "colorpulse":
		r.Colorpulse(start)
	case "rotator":
		r.Rotator(start)
	case "wave":
		r.Wave(start)
	case "storm":
		r.Storm()
	case "flickering":
		r.Flickering(start)
	case "flame":
		r.Flame(start)
	case "aurora":
		r.Aurora(start)
	case "cyberpunkglitch":
		r.CyberpunkGlitch(start)
	case "tokyonight":
		r.TokyoNight(start)
	case "colorshift":
		r.Colorshift(start, owner)
	case "colorwarp":
		r.RGBBrightness = 1
		r.Colorwarp(start, owner)
		rgb.ModifyBrightnessSlice(r.Output, float64(value.brightness)/100)
	default:
		return nil
	}
	if len(r.Output) != 6 {
		return nil
	}
	return append([]byte(nil), r.Output[:3]...)
}

const lightingEndpointAttempts = 3

// Existing direct-USB recovery point only: polling command (when requested),
// then the existing RGB endpoint-open command. No hardware-mode reset, receiver
// rebinding or background reconnect/retry manager is introduced.
// With a rate, the result reports the polling command; endpoint readiness is
// published independently. Without a rate, the result reports endpoint setup.
func (d *Device) recoverCanonicalLightingEndpoint(rate *byte) bool {
	d.lightingMutation.Lock()
	d.lightingMu.Lock()
	if d.lightingStopped || d.lightingRecovering || (rate != nil && d.DeviceProfile == nil) {
		d.lightingMu.Unlock()
		d.lightingMutation.Unlock()
		return false
	}
	d.lightingEndpointAttempted = true
	resume := d.lightingStarted || rate != nil
	d.lightingRecovering = true
	d.lightingHardwareReady = false
	d.retireLightingOwnerLocked(d.owner.kind == lightingCluster, d.owner.kind == lightingOpenRGB)
	d.lightingMu.Unlock()
	if rate != nil {
		// Use the existing profile-switch serializer only for preference publication.
		// Capture/save before releasing it; a switch during the recovery delay must
		// not receive the previous profile's polling preference.
		d.DeviceProfile.PollingRate = int(*rate)
		d.saveDeviceProfile() // Retain existing polling-save failure tolerance.
	}
	d.refreshLightingRegistrationsLocked()
	d.lightingMutation.Unlock()
	ready := false
	defer func() {
		d.lightingMu.Lock()
		d.lightingRecovering = false
		d.lightingHardwareReady = ready && !d.lightingStopped
		if d.lightingHardwareReady && resume {
			d.refreshCanonicalLightingLocked(false)
		}
		d.lightingMu.Unlock()
		d.refreshLightingRegistrations()
	}()
	if rate != nil {
		if d.lightingBeforePolling != nil {
			d.lightingBeforePolling()
		}
		// Legacy intentionally stopped RGB and allowed40ms to settle before polling.
		// Preserve that window without permanently setting Exit on a live USB object.
		if !lightingDelay(d.lightingStop, 40*time.Millisecond) {
			return false
		}
	}
	if !lockLightingOutput(&d.deviceLock, d.lightingStop, d.lightingStop) {
		return false
	}
	defer d.deviceLock.Unlock()
	pollingOK := true
	if rate != nil {
		if _, err := d.transfer(cmdSetPollingRate, []byte{*rate}, "UpdatePollingRate"); err != nil {
			logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to set mouse polling rate")
			// The request may have reached hardware despite a response-read error.
			// Recover its existing RGB endpoint, while still returning polling failure.
			pollingOK = false
		}
	}
	for attempt := 0; attempt < lightingEndpointAttempts; attempt++ {
		select {
		case <-d.lightingStop:
			return rate != nil && pollingOK
		default:
		}
		_, err := d.transfer(cmdOpenEndpoint, nil, "initLeds")
		if err == nil {
			ready = true
			return pollingOK
		}
		logger.Log(logger.Fields{"error": err, "serial": d.Serial}).Error("Unable to open color endpoint")
		if attempt+1 < lightingEndpointAttempts && !lightingDelay(d.lightingStop, 40*time.Millisecond) {
			return rate != nil && pollingOK
		}
	}
	return rate != nil && pollingOK
}
