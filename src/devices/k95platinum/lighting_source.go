package k95platinum

import (
	"fmt"
	"path/filepath"
	"sync"

	"LumenForge/src/config"
	"LumenForge/src/lightingsettings"
)

type k95ResolvedLighting struct { selectedEffect string; brightness uint8; settings lightingsettings.EffectSettings }
type k95SchedulerBrightnessOverride struct { mu sync.RWMutex; value *uint8 }
func (o *k95SchedulerBrightnessOverride) set(value *uint8) bool { o.mu.Lock(); defer o.mu.Unlock(); if o.value == nil && value == nil { return false }; if o.value != nil && value != nil && *o.value == *value { return false }; if value == nil { o.value = nil } else { copy := *value; o.value = &copy }; return true }
func (o *k95SchedulerBrightnessOverride) effective(value uint8) uint8 { o.mu.RLock(); defer o.mu.RUnlock(); if o.value == nil { return value }; return *o.value }

type k95LightingSource interface { resolve() (k95ResolvedLighting, error); resolveEffectSettings(string) (lightingsettings.EffectSettings, error); resolveEffectSettingsWithStatus(string) (lightingsettings.Resolution, error); selectedEffect() (string, error); setSelectedEffect(string) error; setEffectSettings(string, lightingsettings.EffectSettings) error; deleteEffectSettings(string) (bool, error); brightness() (uint8, error); setBrightness(uint8) error }
type k95StateAccess interface { Resolve(string) (lightingsettings.IndependentDeviceLightingState, bool, error); Set(string, lightingsettings.IndependentDeviceLightingState) error }
type k95Resolver interface { Resolve(lightingsettings.Target, string) (lightingsettings.Resolution, error) }
type k95Effects interface { Set(string, string, lightingsettings.EffectSettings) error }
type k95EffectsDelete interface { Delete(string, string) (bool, error) }
type independentK95LightingSource struct { deviceID string; state k95StateAccess; effects k95Effects; resolver k95Resolver }
func (s independentK95LightingSource) selectedEffect() (string,error) { if s.state == nil { return "", fmt.Errorf("K95 canonical lighting runtime is unavailable") }; state,_,err:=s.state.Resolve(s.deviceID); return state.SelectedEffect,err }
func (s independentK95LightingSource) brightness() (uint8,error) { if s.state == nil { return 0, fmt.Errorf("K95 canonical lighting runtime is unavailable") }; state,_,err:=s.state.Resolve(s.deviceID); return state.Brightness,err }
func (s independentK95LightingSource) setSelectedEffect(effect string) error { if s.state == nil { return fmt.Errorf("K95 canonical lighting runtime is unavailable") }; state,_,err:=s.state.Resolve(s.deviceID); if err != nil{return err}; state.SelectedEffect=effect; return s.state.Set(s.deviceID,state) }
func (s independentK95LightingSource) setBrightness(value uint8) error { if s.state == nil { return fmt.Errorf("K95 canonical lighting runtime is unavailable") }; state,_,err:=s.state.Resolve(s.deviceID); if err != nil{return err}; state.Brightness=value; return s.state.Set(s.deviceID,state) }
func (s independentK95LightingSource) resolveEffectSettingsWithStatus(effect string) (lightingsettings.Resolution,error) { if s.resolver == nil{return lightingsettings.Resolution{},fmt.Errorf("K95 canonical lighting runtime is unavailable")}; r,err:=s.resolver.Resolve(lightingsettings.IndependentDevice(s.deviceID),effect); r.Settings=r.Settings.Clone(); return r,err }
func (s independentK95LightingSource) resolveEffectSettings(effect string) (lightingsettings.EffectSettings,error) { r,err:=s.resolveEffectSettingsWithStatus(effect); return r.Settings.Clone(),err }
func (s independentK95LightingSource) setEffectSettings(effect string, settings lightingsettings.EffectSettings) error { if s.effects == nil{return fmt.Errorf("K95 effect customization store is unavailable")}; return s.effects.Set(s.deviceID,effect,settings) }
func (s independentK95LightingSource) deleteEffectSettings(effect string) (bool,error) { store,ok:=s.effects.(k95EffectsDelete); if !ok{return false,fmt.Errorf("K95 effect customization deletion is unavailable")}; return store.Delete(s.deviceID,effect) }
func (s independentK95LightingSource) resolve() (k95ResolvedLighting,error) { state,_,err:=s.state.Resolve(s.deviceID); if err != nil{return k95ResolvedLighting{},err}; if state.SelectedEffect == "keyboard" { return k95ResolvedLighting{selectedEffect: state.SelectedEffect,brightness:state.Brightness},nil }; settings,err:=s.resolveEffectSettings(state.SelectedEffect); return k95ResolvedLighting{selectedEffect:state.SelectedEffect,brightness:state.Brightness,settings:settings},err }

func (d *Device) attachIndependentDeviceLightingRuntime(paths config.Paths) error { runtime,err:=lightingsettings.LoadIndependentDeviceRuntime(paths.OpenRGBDeviceLightingFile,paths.DeviceEffectSettingsFile,filepath.Join(paths.ShippedDatabaseRoot,"rgb.json")); if err != nil{return err}; return d.attachIndependentDeviceLightingSource(runtime) }
func (d *Device) attachIndependentDeviceLightingSource(runtime *lightingsettings.IndependentDeviceRuntime) error { if d == nil || runtime == nil || runtime.State == nil || runtime.Effects == nil || runtime.Resolver == nil{return fmt.Errorf("K95 canonical lighting runtime is unavailable")}; source:=independentK95LightingSource{deviceID:d.Serial,state:runtime.State,effects:runtime.Effects,resolver:runtime.Resolver}; if _,err:=source.resolve();err != nil{return err}; d.lightingSource=source; return nil }
func (d *Device) resolveEffectiveCanonicalLighting() (k95ResolvedLighting,error) { if d == nil || d.lightingSource == nil{return k95ResolvedLighting{},fmt.Errorf("K95 canonical lighting source is unavailable")}; value,err:=d.lightingSource.resolve(); value.brightness=d.schedulerBrightnessOverride.effective(value.brightness); return value,err }
func (d *Device) currentCanonicalSelectedEffect() (string,error) { if d == nil || d.lightingSource == nil{return "",fmt.Errorf("K95 canonical lighting source is unavailable")}; return d.lightingSource.selectedEffect() }
func (d *Device) currentCanonicalBrightness() (uint8,error) { if d == nil || d.lightingSource == nil{return 0,fmt.Errorf("K95 canonical lighting source is unavailable")}; return d.lightingSource.brightness() }
func (d *Device) setCanonicalSelectedEffect(effect string) error { if d == nil || d.lightingSource == nil{return fmt.Errorf("K95 canonical lighting source is unavailable")}; return d.lightingSource.setSelectedEffect(effect) }
func (d *Device) setCanonicalBrightness(value uint8) error { if d == nil || d.lightingSource == nil{return fmt.Errorf("K95 canonical lighting source is unavailable")}; return d.lightingSource.setBrightness(value) }
func (d *Device) restartCanonicalLighting() { if d.lightingRestart != nil { d.lightingRestart(); return }; if d.activeRgb != nil { d.activeRgb.Exit <- true; d.activeRgb=nil }; d.setDeviceColor() }
