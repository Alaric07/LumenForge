# Native Lighting Migration Matrix

## Purpose

This document is the package-level safety inventory for native RGB migration.

The migration must preserve support for every currently supported native device.
A package remains on the legacy RGB implementation until equivalent canonical
lighting behavior is implemented and validated. Matching architectural
signatures are audit aids only; they do not by themselves prove that packages
are safe to migrate together.

## Status definitions

- **Legacy** — current legacy RGB implementation remains authoritative.
- **Audit Required** — the package only matched weak lighting markers and must
  be inspected manually before deciding whether it is a migration target.
- **Audited** — current lighting behavior and hardware-specific capabilities
  have been documented.
- **Migration Ready** — canonical replacement behavior and validation
  requirements are defined.
- **Migrating** — canonical lighting is authoritative for part of the package,
  but replacement UI or retained legacy compatibility dependencies still
  prevent the package from being considered fully migrated.
- **Migrated** — the package/family uses canonical lighting and no longer
  participates in the corresponding legacy lighting path.
- **Validated** — automated parity has passed and hardware testing has been
  completed where hardware is available.
- **Deferred** — intentionally remains on legacy because safe parity has not yet
  been established.
- **Not a lighting target** — manual audit proved the package does not own a
  native lighting implementation requiring migration.

## Migration invariants

A native package must not lose existing supported behavior during migration.

Before a migrated package can be considered complete, its applicable behavior
must be accounted for, including:

- selected effect and renderer behavior;
- Brightness semantics;
- device-specific lighting modes;
- zone, per-key, per-channel, or per-LED behavior;
- RGB Cluster membership and output where supported;
- OpenRGB target-server integration where supported;
- scheduler/lights-out and Off behavior;
- non-lighting user-profile behavior;
- restart, reconnect, and profile-switch behavior;
- persistence and rollback behavior;
- automated regression coverage;
- real-hardware validation where hardware is available.

The legacy `/rgb` editor and remaining global RGB mutation infrastructure stay
available for unmigrated packages until every remaining consumer has parity.
`eligibleForLegacyGlobalRGB()` is the temporary migration bridge between those
two states: it admits unmigrated native packages to the retained global path and
excludes OpenRGB-imported and Cluster devices as before. A native canonical
Lighting provider is excluded only while its `LightingSnapshot()` and runtime
are usable; a structurally present provider whose runtime did not attach falls
back to retained `/rgb` rather than stranding lighting. This is a runtime
boundary, not proof that every legacy-looking package-local helper has already
been deleted. A migrated package can temporarily retain
helpers such as `GetRgbProfiles`, `GetRgbProfile`, `loadRgb`, or
`saveRgbProfile` without making them authoritative lighting state or allowing
the global `/rgb` path to call them. Once no legitimate native `/rgb` consumers
remain, the bridge and related retained compatibility machinery can be removed
with the global system.

### Completed native migration proofs

`scimitarprorgb`, `scimitarrgbelite`, `mm800`, `k95platinum`, `ccxt`, `cc`, and
`memory` are now tracked as **Migrated**. Canonical Device Lighting is
authoritative and these packages no longer participate in retained legacy `/rgb` lighting persistence
or mutation paths.

Completed shared native work includes:

- shared independent-device canonical selected-effect state and complete
  effect-settings resolution;
- canonical desired Brightness;
- reusable native Devices -> Lighting presentation and mutation contracts;
- generic effect, Brightness, Speed, palette-setting, and selected-effect Reset
  mutations without routing native devices through OpenRGB-import endpoints;
- shared authored-zone presentation and mutation for device-owned native modes
  (`ce890f75`).

Scimitar Pro RGB established the first canonical native proof. Scimitar RGB
Elite now uses the same canonical model and exposes its device-authored `mouse`
mode as Front, Scroll, Side, and Logo zones while leaving DPI outside the
generic authored-zone editor.

K95 Platinum is a separate keyboard proof: canonical selected effect, desired
Brightness, generic effect settings, renderer input, and restart behavior are
authoritative while its existing keyboard protocol, per-key state, keyboard
presets, RGB Cluster behavior, and lifecycle remain device-owned. Its `keyboard`
mode is edited in the Keyboard workspace rather than through the generic
authored-zone editor. The inventory records no OpenRGB target-server integration
for this package.

K95 Platinum retains some legacy-looking RGB helpers, but its canonical
presentation provider causes `eligibleForLegacyGlobalRGB()` to reject it before
the global `/rgb` collector or mutation paths can invoke those helpers.

MM800 now uses canonical native Device Lighting and exposes its 15-zone
device-authored `mousepad` mode through the same authored-zone editor. Legacy
row grouping and overlapping legacy layout coordinates remain internal profile
metadata and are not exposed as meaningful shared presentation semantics.

Commander Core XT and Commander CORE establish the separate multi-channel
controller proof. Both expose modern Overview, Lighting, and Cooling workspaces,
with full Device Profiles on Overview, shared Cooling presentation, controller
Brightness, per-channel canonical effects/settings, RGB labels, and Native,
OpenRGB, or RGB Cluster ownership. Their stable canonical children are physical
controller channels; generated topology-derived CCXT 3-pin children are not
stable canonical targets. CCXT keeps its backend-owned 3-Pin RGB Port topology
and `probe-temperature` capability. Commander CORE keeps pump/AIO RGB on real
channel 0 where present, `liquid-temperature`, and its FreeLedPorts-based Custom
RGB Device fallback. Existing RGB Override compatibility may remain in package
code but is not authoritative for migrated canonical channels.

Commander CORE's literal `default` full Device Profile retains cooling, RGB,
labels, CustomLED fallback, ownership, and optional LCD state. Canonical child
effect selections hydrate renderer-facing `RgbDevices[channel].RGB`; structurally
valid selections made unsupported by hardware changes fall back safely to the
canonical default. Its existing CPU/GPU temperature effects remain supported.

Available Commander Core XT and Commander CORE controller, cooling, and lighting
paths received hardware/browser validation. Commander CORE's optional LCD
Display path has automated backend/frontend coverage but was not physically
LCD-validated because no supported LCD-equipped AIO was available.

Memory establishes the separate multi-DIMM + indexed-per-LED proof. Each
physical DIMM is a stable canonical child keyed by its physical `ChannelId`,
with target IDs of `<serial>-rgb-<ChannelId>`. Parent Brightness and Native,
OpenRGB, or RGB Cluster ownership remain device-wide, while each DIMM owns its
selected effect and generic effect settings. The device-authored `led` mode
retains Memory's existing indexed `RGBPerLed` state and is edited through the
modern per-DIMM LED editor with local draft selection, multi-select, Set All,
and one explicit full-palette Save. Legacy `RGBOverride` data is not
canonical-authoritative for migrated DIMMs.

Memory, Commander Core XT, and Commander CORE also expose an aggregate
presentation-only Device Effect control above parent Brightness. A real effect
selected there is validated and applied across existing canonical children;
when child selections differ, the UI reports `Mixed` with a dedicated icon.
`Mixed` is not a persisted or renderer-resolved effect. Memory excludes its
per-DIMM-only `led` mode from aggregate choices.

For authored-zone modes, desired colors remain device-owned state rather than
generic `EffectSettings`. Mutations validate the complete selection before
changing state, persist device-owned authored colors, restart local output only
while the device owns lighting, and suppress local ordinary-zone output while
RGB Cluster or retained OpenRGB integration owns the device.

Other native packages remain separate **Legacy** migration targets. No parity
is inferred from similar package names, device shape, or matching audit
signatures.


Device-workspace migration is tracked separately from this lighting matrix.
Scimitar RGB Elite's modern DPI, Performance, Key Assignments, and physical-button
assignment work does not alter its lighting status. Likewise, adding a shared
Performance or Overview provider to another native package must not be treated
as evidence that its lighting implementation has migrated or that unowned
hardware has been physically validated.

---

# LumenForge Native Lighting Migration Inventory

Generated as a read-only architecture inventory. This does not declare migration parity or hardware support status.

## Summary

- Packages with any scanned lighting marker: **137**
- Strong legacy-lighting candidates: **131**
- Weak-marker-only packages requiring manual review: **6**

### Structural shapes

- zoned: **64**
- single-profile: **52**
- multi-channel: **9**
- weak-marker only: **6**
- multi-channel + per-LED: **4**
- other lighting: **2**

## Package matrix

| Package | Strong | Status | Shape | Legacy RGB | Brightness | Override | Zones | Per LED | Cluster | OpenRGB target | Special modes |
|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| `cc` | Y | Migrated | multi-channel |  | Y | Y |  |  | Y | Y | led, liquid-temperature |
| `ccxt` | Y | Migrated | multi-channel |  | Y | Y |  |  | Y | Y | probe-temperature |
| `cduo` | Y | Legacy | multi-channel | Y | Y | Y |  |  | Y | Y | probe-temperature |
| `clipperpromini60` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `cone` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led, liquid-temperature |
| `cpro` | Y | Legacy | multi-channel | Y | Y |  |  |  | Y | Y |  |
| `darkcorergbproW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproseW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproseWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbseW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `darkcorergbseWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `darkstarW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `darkstarWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `elite` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led, liquid-temperature |
| `glaivergb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `glaivergbpro` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `harpoonW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `harpoonWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `harpoonrgbpro` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `hs80maxW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `hs80rgb` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `hs80rgbW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `hs80rgbWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `hydro` | Y | Legacy | multi-channel | Y | Y |  |  |  |  |  |  |
| `ironclaw` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawSEW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawSEWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `k100` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k100airW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k100airWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k55` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k55core` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k55coretkl` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k55pro` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k55proXT` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `k57rgbW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k57rgbWU` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `k60rgbpro` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `k65plusW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k65plusWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k65pm` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k65rgb` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k65rgbRF` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k65rm` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k68rgb` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70core` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70coretkl` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70coretklW` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `k70coretklWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70lux` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70luxrgb` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70max` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70mk2` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70pmW` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k70pmWU` | Y | Legacy | single-profile | Y | Y |  |  |  | Y |  | keyboard |
| `k70pro` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70protkl` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k70rgbRF` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k70rgbtklcs` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `k95` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `k95platinum` | Y | Migrated | single-profile |  | Y |  |  |  | Y |  | keyboard |
| `k95platinumXT` | Y | Legacy | single-profile | Y |  |  |  |  |  |  | keyboard |
| `katarpro` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `katarproW` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  |  |
| `katarproxt` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `lncore` | Y | Legacy | multi-channel | Y | Y |  |  |  |  |  |  |
| `lnpro` | Y | Legacy | multi-channel | Y | Y |  |  |  |  |  |  |
| `lsh` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led, liquid-temperature, probe-temperature |
| `lt100` | Y | Legacy | multi-channel | Y | Y |  |  |  | Y | Y |  |
| `m55` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  |  |
| `m55W` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  |  |
| `m55rgbpro` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65prorgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbelite` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultra` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultraW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultraWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m75` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `m75AirW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `m75AirWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `m75W` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `m75WU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `makr75W` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `makr75WU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `memory` | Y | Migrated | multi-channel + per-LED |  | Y | Y |  | Y | Y | Y | led |
| `mm700` | Y | Legacy | single-profile | Y | Y |  |  |  | Y | Y | mousepad |
| `mm800` | Y | Migrated | single-profile |  | Y |  |  |  | Y | Y | mousepad |
| `motherboard` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `nautilusLcd` | Y | Legacy | other lighting | Y |  |  |  |  |  |  |  |
| `nexus` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `nightsabreW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `nightsabreWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `nightswordrgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `platinum` | Y | Legacy | multi-channel | Y | Y |  |  |  |  | Y | liquid-temperature |
| `psudongle` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `psuhid` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `sabreprocs` | Y | Legacy | other lighting | Y |  |  |  |  |  |  |  |
| `sabrergbpro` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrergbproW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrergbproWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrev2proW` | Y | Legacy | zoned |  | Y |  | Y |  |  |  | mouse |
| `sabrev2proWU` | Y | Legacy | zoned |  | Y |  | Y |  |  |  | mouse |
| `scimitar` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarSEW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarSEWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarprorgb` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `scimitarrgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarrgbelite` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `scufenvisionproV2W` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `scufenvisionproV2WU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `scufenvisionproW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `scufenvisionproWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `slipstream` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `st100` | Y | Legacy | single-profile | Y | Y |  |  |  | Y | Y | stand |
| `strafergbmk2` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `vanguard96` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96W` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96WU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96pro` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard99airW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard99airWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `virtuosoSEW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosoSEWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosoW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosoWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosomaxW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosorgbXTW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosorgbXTWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `voidV2W` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `voideliteW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `xc7` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | liquid-temperature |
| `xeneonedge` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |

## Weak-marker audit

The six weak-marker packages were inspected manually after the initial
inventory. None owns a native RGB lighting implementation requiring canonical
lighting migration:

- `motherboard` manages motherboard fan/header behavior. Its `RgbOff` profile
  field does not correspond to a package-owned RGB lighting engine.
- `nexus` is an LCD/touch-screen target. RGB color values are used for display
  presentation such as button and text colors, not native device lighting.
- `psudongle` and `psuhid` manage PSU telemetry and fan behavior and do not own
  RGB lighting implementations.
- `xeneonedge` manages XENEON EDGE display widgets and does not own an RGB
  lighting implementation.
- `slipstream` is a transport/host for paired wireless devices rather than a
  lighting target itself. Paired device packages remain independent native
  lighting migration targets and must retain Slipstream operation during their
  migrations.

These packages are therefore classified as **Not a lighting target**. Their
non-lighting behavior remains supported and must not be disturbed by native
lighting migration work.

## Architectural signature groups

These groups are only an audit shortcut. Matching signatures do NOT mean packages are automatically safe to migrate together.

### zoned + cluster + openrgb-target + special:mouse

Count: **35**

`darkcorergbproW`, `darkcorergbproWU`, `darkcorergbproseW`, `darkcorergbproseWU`, `darkstarW`, `darkstarWU`, `glaivergb`, `glaivergbpro`, `harpoonW`, `harpoonWU`, `ironclaw`, `ironclawSEW`, `ironclawSEWU`, `ironclawW`, `ironclawWU`, `m55rgbpro`, `m65prorgb`, `m65rgbelite`, `m65rgbultra`, `m65rgbultraW`, `m65rgbultraWU`, `nightsabreW`, `nightsabreWU`, `nightswordrgb`, `sabrergbpro`, `sabrergbproW`, `sabrergbproWU`, `scimitar`, `scimitarSEW`, `scimitarSEWU`, `scimitarW`, `scimitarWU`, `scimitarprorgb`, `scimitarrgb`, `scimitarrgbelite`

### single-profile + cluster + special:keyboard

Count: **27**

`clipperpromini60`, `k100`, `k100airW`, `k100airWU`, `k57rgbW`, `k65plusW`, `k65plusWU`, `k65pm`, `k65rm`, `k70core`, `k70coretkl`, `k70coretklWU`, `k70max`, `k70pmW`, `k70pmWU`, `k70pro`, `k70protkl`, `k70rgbtklcs`, `k95platinum`, `makr75W`, `makr75WU`, `vanguard96`, `vanguard96W`, `vanguard96WU`, `vanguard96pro`, `vanguard99airW`, `vanguard99airWU`

### zoned

Count: **19**

`hs80maxW`, `hs80rgb`, `hs80rgbW`, `hs80rgbWU`, `m75AirW`, `m75AirWU`, `scufenvisionproV2W`, `scufenvisionproV2WU`, `scufenvisionproW`, `scufenvisionproWU`, `virtuosoSEW`, `virtuosoSEWU`, `virtuosoW`, `virtuosoWU`, `virtuosomaxW`, `virtuosorgbXTW`, `virtuosorgbXTWU`, `voidV2W`, `voideliteW`

### single-profile + special:keyboard

Count: **18**

`k55`, `k55core`, `k55coretkl`, `k55pro`, `k55proXT`, `k57rgbWU`, `k60rgbpro`, `k65rgb`, `k65rgbRF`, `k68rgb`, `k70coretklW`, `k70lux`, `k70luxrgb`, `k70mk2`, `k70rgbRF`, `k95`, `k95platinumXT`, `strafergbmk2`

### zoned + special:mouse

Count: **10**

`darkcorergbseW`, `darkcorergbseWU`, `harpoonrgbpro`, `katarpro`, `katarproxt`, `m75`, `m75W`, `m75WU`, `sabrev2proW`, `sabrev2proWU`

### weak-marker only

Count: **6**

`motherboard`, `nexus`, `psudongle`, `psuhid`, `slipstream`, `xeneonedge`

### multi-channel

Count: **3**

`hydro`, `lncore`, `lnpro`

### single-profile

Count: **3**

`katarproW`, `m55`, `m55W`

### multi-channel + cluster + openrgb-target

Count: **2**

`cpro`, `lt100`

### multi-channel + override + cluster + openrgb-target + special:probe-temperature

Count: **2**

`ccxt`, `cduo`

### multi-channel + per-LED + override + cluster + openrgb-target + special:led,liquid-temperature

Count: **2**

`cone`, `elite`

### other lighting

Count: **2**

`nautilusLcd`, `sabreprocs`

### single-profile + cluster + openrgb-target + special:mousepad

Count: **2**

`mm700`, `mm800`

### multi-channel + openrgb-target + special:liquid-temperature

Count: **1**

`platinum`

### multi-channel + override + cluster + openrgb-target + special:led,liquid-temperature

Count: **1**

`cc`

### multi-channel + per-LED + override + cluster + openrgb-target + special:led

Count: **1**

`memory`

### multi-channel + per-LED + override + cluster + openrgb-target + special:led,liquid-temperature,probe-temperature

Count: **1**

`lsh`

### single-profile + cluster + openrgb-target + special:stand

Count: **1**

`st100`

### single-profile + special:liquid-temperature

Count: **1**

`xc7`
