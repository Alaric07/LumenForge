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
- **Dormant/inert Lighting metadata** — RGB-shaped persisted fields or markers
  exist, but audit found no RGB profile store, Lighting mutation, renderer, or
  physical LED/output path. It is not a migration target.
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
Repository cleanup must not remove package-local or shared legacy UI/runtime
dependencies for a device while that package remains Legacy, Audit Required,
Deferred, or otherwise depends on the retained legacy path. This matrix is the
package-level authority for that safety decision; the roadmap carries the wider
classification policy.
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

`scimitarprorgb`, `scimitarrgbelite`, `mm800`, `k95platinum`, `ccxt`, `cc`,
`memory`, `st100`, `m75`, `m75W`, `m75WU`, `hydro`, `xc7`, `katarpro`,
`darkcorergbproWU`, and `darkcorergbproseWU` are now tracked as **Migrated**. Canonical Device Lighting is
authoritative while the canonical runtime is usable, and these packages then
no longer participate in retained legacy `/rgb` lighting persistence or mutation
paths.

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

ST100 RGB is the canonical authored-zone accessory reference. Its modern Devices
workspace and canonical Lighting migration preserve device zone IDs and packet
mappings while adapting its 9-zone stand geometry for the shared presentation
layer. `stand` remains the device-authored mode, `static` uses the canonical
single-color editor, and its source-backed effects remain selectable. Brightness
and RGB Cluster ownership use existing device mutations; this does not imply
that other lighting accessories have migrated.

M75 W and M75 WU are the focused wireless-mouse sibling proof
(`90f1d10e`, *Migrate M75 wireless lighting*). Both use canonical selected
effect and desired Brightness with their 19 existing software-rendered effects.
Their device-authored `mouse` mode keeps Bottom and Logo colors in the existing
device-owned `ZoneColors` persistence and output mapping. Neither package
supports RGB Cluster or native OpenRGB Integration. Automated and source-backed
validation is complete; no physical M75 hardware validation has been recorded.
Their shared migration followed focused package-equivalence evidence and does
not make other W/WU siblings safe to batch.

Wired M75 (`src/devices/m75`) completed its standalone canonical migration in
`749f781f` (*Migrate wired M75 lighting*) with the exact source-backed catalogue
of 20 effects, including authored `mouse` mode with Bottom / Logo zones.
Canonical state owns selected effect and desired Brightness; generic effect
settings use canonical `lightingsettings.DeviceStore`. Scheduler darkness and
user RGB-off are independent transient overrides. It has no RGB Cluster or
OpenRGB Integration capability. Wired HID/output/lifecycle behavior is
preserved, and canonical runtime failure retains the legacy Lighting fallback.
Its canonical preview is migrated and inert. Automated package/server/race
validation passed for the migration; no physical hardware validation was
performed. The M75 family now has canonical coverage for wired M75, M75 W,
and M75 WU, excluding M75 Air W/WU.

XC7 (`src/devices/xc7`) completed its standalone canonical Lighting migration
in `2e948d43` (*Migrate XC7 lighting*), separately from its earlier modern
workspace, Display, and liquid-temperature telemetry work. Canonical Lighting
is now presentation/state authority over the existing XC7 device/RGB profile
persistence; no second RGB database was introduced. The supported XC7 ELITE LCD
product retains its 31-LED topology and existing physical renderer/output path,
HID/LCD/temperature transport, and separate Display/telemetry behavior.

Its exact catalogue contains **23 selectable effects**: `circle`, `circleshift`,
`colorpulse`, `colorshift`, `colorwarp`, `cpu-temperature`, `flickering`, `flame`,
`aurora`, `cyberpunkglitch`, `tokyonight`, `gpu-temperature`, `gradient`,
`liquid-temperature`, `off`, `rainbow`, `pastelrainbow`, `rotator`, `spinner`,
`static`, `storm`, `watercolor`, and `wave`. The internal renderer `custom`
branch is not selectable. Liquid/CPU/GPU temperature effects retain their
existing XC7/CPU/GPU sources and profile `MinTemp` / `MaxTemp` settings.
`BrightnessSlider` remains desired 0..100 renderer Brightness, with the existing
`GlobalBrightness` guard. Scheduler darkness and user RGB-off are transient
runtime overrides; clearing them restores the latest desired state. Profile
switching refreshes canonical state and clears stale persisted RGB-off state.
Disconnected, stopped, or missing-runtime canonical mutations fail closed before
state, persistence, renderer, or hardware changes. Attachment failure leaves
Display/telemetry usable; LegacyLighting is suppressed only when a usable
canonical snapshot resolves, retaining the established fallback otherwise.
There are no authored zones, RGB Cluster, or native OpenRGB Integration controls.
The canonical modern preview is inert: no device initialization, HID access,
persistence mutation, or renderer goroutines. Focused package/server tests,
race testing, and `git diff --check` passed; CodeRabbit reported no new findings.
No physical hardware validation was performed.

Wired KATAR PRO (`src/devices/katarpro`) completed its standalone canonical
Lighting migration in `17631e14` (*Migrate KATAR PRO lighting*), separately from
its modern Devices workspace migration. Canonical Lighting is presentation/state
authority over the existing `database/rgb/<serial>.json` effect-settings store
and active device profile backing state for selected effect, desired Brightness,
authored zone color, and profile selection. No second RGB database or persistence
format was introduced; the existing renderer/output/HID path remains authoritative.

Its exact catalogue contains **19 selectable effects**: `colorpulse`, `colorwarp`,
`cpu-temperature`, `flickering`, `flame`, `aurora`, `cyberpunkglitch`, `tokyonight`,
`gpu-temperature`, `gradient`, `mouse`, `off`, `rainbow`, `pastelrainbow`, `rotator`,
`static`, `storm`, `watercolor`, and `wave`. Shared effects `colorshift`, `circle`,
`circleshift`, and `spinner` remain excluded. The source-backed authored `mouse`
mode retains exactly one zone: **zone 0, Scroll**, with RGB byte mapping
**[0, 1, 2]**. Sniper color substitution and brightness application are runtime-only
and use local copies without mutating persisted Scroll-zone or Sniper DPI colors.
CPU/GPU temperature effects retain their existing sources and profile
`MinTemp` / `MaxTemp` semantics.

Desired Brightness remains the active profile's 0..100 `BrightnessSlider` value.
Scheduler darkness and user RGB-off are independent transient runtime overrides;
neither replaces desired effect, Brightness, or settings, and clearing them
restores the latest desired state. Profile switching refreshes canonical state
and retires stale legacy off state. Canonical mutations persist proposed values
before publishing state or restarting output; persistence failure leaves prior
state/output authoritative. Unavailable/stopped mutations fail closed before
state, persistence, renderer restart, or hardware output changes.

Canonical attachment failure leaves the rest of the mouse workspace usable.
LegacyLighting remains eligible unless a usable canonical snapshot resolves.
The canonical modern preview is inert: no Init, HID, device registration,
persistence, renderer goroutine, or background lifecycle. No RGB Cluster,
OpenRGB, or per-LED capability was added. `katarproW` remains a separate confirmed
non-Lighting target. Focused package/server/race tests and `git diff --check`
passed for the migration; no physical hardware validation was performed.

**G01 — completed Dark Core PRO bulk migration**

`darkcorergbproWU` and `darkcorergbproseWU` completed one source-proven,
same-transport model-revision pass in `438a7d45` (*Migrate Dark Core PRO
lighting*). Both use package-local canonical adapters and expose exactly
**20 effects**: `colorpulse`, `colorshift`, `colorwarp`, `cpu-temperature`,
`flickering`, `flame`, `aurora`, `cyberpunkglitch`, `tokyonight`,
`gpu-temperature`, `gradient`, `mouse`, `off`, `rainbow`, `pastelrainbow`,
`rotator`, `static`, `storm`, `watercolor`, and `wave`.

Their authored `mouse` zones are identical; IDs and RGB byte mappings remain
device-owned:

| Zone ID | Zone | RGB byte mapping |
|---:|---|---|
| 0 | Scroll | [0,12,24] |
| 1 | Logo | [6,18,30] |
| 2 | Side Accent 1 | [1,13,25] |
| 3 | Side Accent 2 | [2,14,26] |
| 4 | Side Accent 3 | [3,15,27] |
| 5 | Side Accent 4 | [4,16,28] |
| 6 | Side Accent 5 | [5,17,29] |
| 7 | Side Accent 6 | [7,19,31] |

Desired `BrightnessSlider` authority, transient scheduler darkness and RGB-off,
persist-before-publication, active-profile refresh, defensive color/settings
copies, and usable-snapshot legacy fallback/suppression are preserved. Cluster
and native OpenRGB ownership remain intact, and canonical previews are inert.
Profile switching normalizes nil legacy `BrightnessSlider` to 100 in both
packages. Per the migration closeout, CodeRabbit reviewed all nine files and
found that one minor inactive-profile switching issue; it was fixed and the
requested package/server/race validation passed. **No physical hardware
validation was performed.** G01 is completed, not a proposed bulk group; its W
counterparts completed the separate receiver-backed G06 pass (`53c19c88`).

**G02–G09 — completed grouped migrations**

G02–G09 are complete canonical Lighting migrations:

| Group | Packages | Completed commit | Pending targets / passes after group |
|---|---|---|---|
| G02 | `ironclawWU` + `ironclawSEWU` | `a20d8968` | 104 / 96 |
| G03 | `scimitarWU` + `scimitarSEWU` | `70d8b746` | 102 / 95 |
| G04 | `virtuosoWU` + `virtuosoSEWU` | `9c27548a` | 100 / 94 |
| G05 | `scufenvisionproW` + `scufenvisionproV2W` | `ed892867` | 98 / 93 |
| G06 | `darkcorergbproW` + `darkcorergbproseW` | `53c19c88` | 96 / 92 |
| G07 | `ironclawW` + `ironclawSEW` | `2a26bdc7` | 94 / 91 |
| G08 | `scimitarW` + `scimitarSEW` | `f43c4c13` | 92 / 90 |
| G09 | `virtuosoW` + `virtuosoSEW` | `08261567` | 90 / 89 |

- G02 (`a20d8968`, *Migrate Ironclaw lighting*): `ironclawWU` +
  `ironclawSEWU` expose the source-backed 20-effect mouse catalogue and three-zone
  topology; Cluster/OpenRGB ownership is preserved.
- G03 (`70d8b746`, *Migrate Scimitar lighting*): `scimitarWU` +
  `scimitarSEWU` expose the source-backed 20-effect mouse catalogue and Side/Logo
  topology; Cluster/OpenRGB is preserved. The renderer-survival regression for
  transient nil DPI composition was fixed.
- G04 (`9c27548a`, *Migrate Virtuoso lighting*): `virtuosoWU` +
  `virtuosoSEWU` expose the source-backed 20-effect headset catalogue and
  Logo / Microphone / Indicator LED zones. No Cluster/OpenRGB capability is
  advertised; headset overlays and presentation are preserved.
- G05 (`ed892867`, *Migrate SCUF Envision Pro lighting*): `scufenvisionproW` +
  `scufenvisionproV2W` expose the source-backed 20-effect Controller catalogue
  and one Controller zone `{0,9,18}`. No `SchedulerBrightness` was invented,
  and no Cluster/OpenRGB capability is advertised. Receiver, analog, trigger,
  and vibration behavior is preserved. Git history resolves the full commit to
  `ed892867c3b7ad199fafdcc31c2a4859eee712c6`; the closeout reported no new
  CodeRabbit findings before commit and push.
- G06 (`53c19c88`, *Migrate Dark Core PRO receiver lighting*):
  `darkcorergbproW` + `darkcorergbproseW` complete the receiver-backed counterpart
  of G01. Source-backed scheduler/RGB-off and Cluster/OpenRGB are preserved,
  including SE package-specific Connect, sleep, and key-assignment behavior.
- G07 (`2a26bdc7`, *Migrate Ironclaw receiver lighting*): `ironclawW` +
  `ironclawSEW` complete the receiver-backed counterpart of G02, preserving
  Cluster/OpenRGB, package-specific Connect, and SE lift-height behavior.
- G08 (`f43c4c13`, *Migrate Scimitar receiver lighting*): `scimitarW` +
  `scimitarSEW` complete the receiver-backed counterpart of G03, preserving
  Cluster/OpenRGB and renderer-survival behavior. CodeRabbit identified a major
  regression that initially dropped scheduler/RGB-off requests while asleep or
  disconnected. Before commit, the fix retained transient `schedulerDark` /
  `userRGBOff` state while canonical attachment/profile exists; physical
  readiness gates only immediate restart/output. Reconnect/wake respects that
  retained state; ordinary authored mutations still require normal readiness.
- G09 (`08261567`, *Migrate Virtuoso receiver lighting*): `virtuosoW` +
  `virtuosoSEW` complete the receiver-backed counterpart of G04 with
  source-confirmed Logo / Microphone / Indicator LED topology. Scheduler/RGB-off
  requests are retained while asleep/disconnected. No Cluster/OpenRGB is
  advertised; receiver/audio/mic/mute/listener behavior remains package-local.

All eight groups are canonical Lighting complete. **No physical hardware
validation was performed for G02–G09.**

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

Other source-confirmed native packages remain separate **Legacy** migration
targets. No parity or target classification is inferred from similar package
names, device shape, RGB-shaped fields, persistence markers, or matching audit
signatures.


Device-workspace migration is tracked separately from this lighting matrix.
The final repository-wide modern device-workspace inventory is complete for
active supported device families: capability-driven presentation is available
where each source contract justifies it. XENEON EDGE is intentionally deferred
because discovery/registration is disabled and its widget/kiosk backend is
incomplete; KDE/display-touch documentation is not a device-control contract.
Transport-only Slipstream, dongle, and receiver packages remain transport-only,
not independent workspaces. None of those workspace classifications changes
this matrix's lighting status.

Modern keyboard, mouse, headset, SCUF controller, cooling, display, and LINK
workspace coverage does not make a package a canonical-lighting migration. K95
Platinum is the canonical-lighting **Migrated** keyboard proof; K100, K100 AIR,
K57, K60, K68, K55, K65, K70, and the other completed keyboard workspace
families remain **Legacy** for lighting unless their individual matrix row says
otherwise. The same applies to completed mouse, headset, SCUF controller, and
LSH workspaces: retained legacy lighting remains authoritative until a separate
family-specific lighting cutover. Package-local modern controls do not change
that status or demonstrate physical lighting validation. The same distinction
also applies to Scimitar RGB Elite's modern DPI, Performance, Key Assignments,
and physical-button assignment work.

---

# LumenForge Native Lighting Migration Inventory

Generated as a read-only architecture inventory. This does not declare migration parity or hardware support status.

## Summary

- Packages with any scanned lighting marker: **137**
- Strong-marker packages requiring source-contract classification: **131**
- Weak-marker-only packages requiring manual review: **6**
- Corrected assessment baseline after excluding `sabreprocs`: **108 A + 0 B =
  108 packages / 98 conservative scheduling passes**, including G01–G09.
- Pending after completed G01–G09: **90 A + 0 B = 90 Lighting targets / 89 passes**;
  G10 is the only remaining original two-package group, plus 88 standalone slots,
  of which 54 still require focused audits.

Strong markers are audit leads, not proof of a Lighting implementation. A real
Lighting target requires source-backed persisted state, a user mutation,
a renderer/direct output resolver, and a device-owned physical write boundary;
wireless packages also require lifecycle/reconnect proof where applicable.
RGB-looking fields, `RGBModes`, or persistence metadata alone do not qualify.
The strict master audit removed the Sabre V2 W/WU shared batch, Nautilus LCD,
the M75 AIR W/WU sibling batch, and Katar Pro W from the backlog. M75 W/WU
subsequently completed their focused canonical migration, reducing the queue by
two packages and one pass. Hydro then completed its constrained fixed-static
canonical migration in `395b23b9`, reducing the queue by one package and one
pass. Wired M75 then completed one standalone A package/pass in `749f781f`,
reducing **114 A + 0 B = 114 packages / 86 tentative passes** to
**113 A + 0 B = 113 packages / 85 tentative passes**. Focused source-contract
audits then removed `m55` and `m55W` as strong-marker false positives. Their
previous two-package `single-profile` scheduling group represented one
tentative migration pass, so removing it yielded
**111 A + 0 B = 111 packages / 84 tentative passes**. XC7 then completed
its standalone A package/pass in `2e948d43`. Its documented
`single-profile + special:liquid-temperature` group contains only `xc7`, so
removing that one remaining package/pass yielded
**110 A + 0 B = 110 packages / 83 tentative passes**. Wired KATAR PRO then
completed its standalone A package/pass in `17631e14`. Its shared
`zoned + special:mouse` signature is a discovery grouping, not a shared migration
pass with `katarproxt` or the other listed packages. Removing `katarpro` alone
therefore yielded the pre-reconciliation queue:
**109 A + 0 B = 109 packages / 82 tentative passes**. The 137/131/6
marker figures above remain historical structural-scan totals, not current
target counts.

### Structural shapes

Historical structural-scan totals; these are not the current audited queue.

- zoned: **60**
- single-profile: **51**
- multi-channel: **9**
- weak-marker only: **6**
- multi-channel + per-LED: **4**
- other lighting: **1**
- dormant/inert metadata: **6**

## Package matrix

| Package | Strong | Status | Shape | Legacy RGB | Brightness | Override | Zones | Per LED | Cluster | OpenRGB target | Special modes |
|---|---:|---|---|---:|---:|---:|---:|---:|---:|---:|---|
| `cc` | Y | Migrated | multi-channel |  | Y | Y |  |  | Y | Y | led, liquid-temperature |
| `ccxt` | Y | Migrated | multi-channel |  | Y | Y |  |  | Y | Y | probe-temperature |
| `cduo` | Y | Legacy | multi-channel | Y | Y | Y |  |  | Y | Y | probe-temperature |
| `clipperpromini60` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `cone` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led, liquid-temperature |
| `cpro` | Y | Legacy | multi-channel | Y | Y |  |  |  | Y | Y |  |
| `darkcorergbproW` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproWU` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproseW` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `darkcorergbproseWU` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
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
| `hydro` | Y | Migrated | multi-channel |  | Y |  |  |  |  |  | fixed static; no effect-selection mutation |
| `ironclaw` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `ironclawSEW` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `ironclawSEWU` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `ironclawW` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `ironclawWU` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
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
| `k60rgbpro` | Y | Deferred | single-profile | Y |  |  |  |  |  |  | keyboard |
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
| `katarpro` | Y | Migrated | zoned |  | Y |  | Y |  |  |  | mouse |
| `katarproW` | Y | Not a lighting target | dormant/inert metadata | Y | Y |  |  |  |  |  | DPI indicator only |
| `katarproxt` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  | mouse |
| `lncore` | Y | Legacy | multi-channel | Y | Y |  |  |  |  |  |  |
| `lnpro` | Y | Legacy | multi-channel | Y | Y |  |  |  |  |  |  |
| `lsh` | Y | Legacy | multi-channel + per-LED | Y | Y | Y |  | Y | Y | Y | led, liquid-temperature, probe-temperature |
| `lt100` | Y | Legacy | multi-channel | Y | Y |  |  |  | Y | Y |  |
| `m55` | Y | Not a lighting target | dormant/inert Lighting metadata | Y | Y |  |  |  |  |  | DPI/Sniper indicator only |
| `m55W` | Y | Not a lighting target | dormant/inert Lighting metadata | Y | Y |  |  |  |  |  | DPI/Sniper indicator only |
| `m55rgbpro` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65prorgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbelite` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultra` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultraW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m65rgbultraWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `m75` | Y | Migrated | zoned |  | Y |  | Y |  |  |  | mouse |
| `m75AirW` | Y | Not a lighting target | dormant/inert metadata | Y | Y |  | Y |  |  |  | DPI/Sniper indicator only |
| `m75AirWU` | Y | Not a lighting target | dormant/inert metadata | Y | Y |  | Y |  |  |  | DPI/Sniper indicator only |
| `m75W` | Y | Migrated | zoned |  | Y |  | Y |  |  |  | mouse |
| `m75WU` | Y | Migrated | zoned |  | Y |  | Y |  |  |  | mouse |
| `makr75W` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `makr75WU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `memory` | Y | Migrated | multi-channel + per-LED |  | Y | Y |  | Y | Y | Y | led |
| `mm700` | Y | Legacy | single-profile | Y | Y |  |  |  | Y | Y | mousepad |
| `mm800` | Y | Migrated | single-profile |  | Y |  |  |  | Y | Y | mousepad |
| `motherboard` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `nautilusLcd` | Y | Not a lighting target | dormant/inert metadata | Y |  |  |  |  |  |  | LCD output only |
| `nexus` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `nightsabreW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `nightsabreWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `nightswordrgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `platinum` | Y | Legacy | multi-channel | Y | Y |  |  |  |  | Y | liquid-temperature |
| `psudongle` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `psuhid` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `sabreprocs` | Y | Not a lighting target | dormant/orphan RGB metadata |  |  |  |  |  |  |  | DPI/Sniper indicator only |
| `sabrergbpro` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrergbproW` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrergbproWU` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `sabrev2proW` | Y | Dormant/inert Lighting metadata | dormant/inert metadata |  | Y |  |  |  |  |  | DPI indicator only |
| `sabrev2proWU` | Y | Dormant/inert Lighting metadata | dormant/inert metadata |  | Y |  |  |  |  |  | DPI indicator only |
| `scimitar` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarSEW` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `scimitarSEWU` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `scimitarW` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `scimitarWU` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `scimitarprorgb` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `scimitarrgb` | Y | Legacy | zoned | Y | Y |  | Y |  | Y | Y | mouse |
| `scimitarrgbelite` | Y | Migrated | zoned |  | Y |  | Y |  | Y | Y | mouse |
| `scufenvisionproV2W` | Y | Migrated | zoned |  | Y |  | Y |  |  |  |  |
| `scufenvisionproV2WU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `scufenvisionproW` | Y | Migrated | zoned |  | Y |  | Y |  |  |  |  |
| `scufenvisionproWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `slipstream` |  | Not a lighting target | weak-marker only |  |  |  |  |  |  |  |  |
| `st100` | Y | Migrated | single-profile |  | Y |  |  |  | Y | Y | stand |
| `strafergbmk2` | Y | Legacy | single-profile | Y | Y |  |  |  |  |  | keyboard |
| `vanguard96` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96W` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96WU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard96pro` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard99airW` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `vanguard99airWU` | Y | Legacy | single-profile | Y |  |  |  |  | Y |  | keyboard |
| `virtuosoSEW` | Y | Migrated | zoned |  | Y |  | Y |  |  |  |  |
| `virtuosoSEWU` | Y | Migrated | zoned |  | Y |  | Y |  |  |  |  |
| `virtuosoW` | Y | Migrated | zoned |  | Y |  | Y |  |  |  |  |
| `virtuosoWU` | Y | Migrated | zoned |  | Y |  | Y |  |  |  |  |
| `virtuosomaxW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosorgbXTW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `virtuosorgbXTWU` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `voidV2W` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `voideliteW` | Y | Legacy | zoned | Y | Y |  | Y |  |  |  |  |
| `xc7` | Y | Migrated | single-profile |  | Y |  |  |  |  |  | liquid-temperature |
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

## Strong-marker/source-contract audit

The same source-contract rule applies to strong markers. `sabrev2proW` and
`sabrev2proWU` persist `RGBProfile`, `BrightnessSlider`,
`OriginalBrightness`, and `RGBModes`, but neither has an RGB profile store, a
Lighting mutation, renderer, LED frame, or physical Lighting output. Their
`rgb.Color` values are DPI-stage indicator state. They are dormant/inert
metadata, not Legacy Lighting targets.

`nautilusLcd` is not a Lighting target: its physical output is LCD
feature-report/image transfer. Its `Rgb` and `saveRgbProfile` are orphaned
RGB-shaped metadata with no renderer, mutation, or LED output consumer.

`m75AirW` and `m75AirWU` remain false positives / non-targets for canonical
Lighting: DPI/Sniper indicator behavior is not a complete general Lighting
contract. Their retained RGB-shaped
state is not rendered; the only physical color output is the active DPI/Sniper
indicator. `katarproW` is likewise not a Lighting target: it has no RGB profile
store or Lighting mutation and flashes a DPI-stage indicator before returning it
to black. These packages are not candidate sibling/standalone migrations.

`m55` and `m55W` are strong-marker false positives, classified as **Not a
lighting target | dormant/inert Lighting metadata | DPI/Sniper indicator only**.
Both have `LEDChannels = 1` and `ChangeableLedChannels = 0`. Physical color
output is the active DPI-stage color, with Sniper mode substituting the Sniper
DPI color. `BrightnessSlider` scales that indicator, and `setDeviceColor` writes
one RGB triplet. Neither initializes a package-owned general effect
renderer/runtime or has a selectable Lighting effect pipeline reaching physical
output. Their `RGBProfile="mouse"`, `Rgb *rgb.RGB`, `RgbOff`, and legacy-looking
RGB mutation scaffolding are dormant/non-authoritative for physical Lighting
behavior; they do not establish canonical Lighting support.

If a package does not have a complete source-backed Lighting contract, the
modern workspace must not advertise a Lighting tab/workspace for it.

This fail-closed rule applies to confirmed non-targets including `m55`, `m55W`,
`m75AirW`, `m75AirWU`, `katarproW`, `sabreprocs`, `sabrev2proW`, `sabrev2proWU`, and
`nautilusLcd`. DPI/Sniper indicator color belongs with mouse/DPI presentation;
LCD-related color/output belongs with display presentation. Telemetry,
transport, and other non-lighting color state must remain in their proper
feature areas and must not cause Lighting capability exposure. Indicator colors
are not general Lighting features. Removing any exposed modern Lighting
tabs/workspaces for confirmed non-targets is required follow-up cleanup; this
docs-only reconciliation does not implement UI or runtime changes.

`hydro` completed its constrained fixed-static canonical migration in
`395b23b9` (*Migrate Hydro static lighting*). It has canonical fixed `static`
identity, editable Static color, and canonical desired Brightness; it has no
effect-selection mutation, Speed, authored zones, RGB Cluster, or native
OpenRGB Integration. Scheduler darkness is transient. The direct existing HID
configuration boundary remains intact. Automated/source-backed validation is
complete; no physical Hydro hardware validation is recorded.

K60 RGB PRO (`k60rgbpro`) remains a complete-contract **A** Lighting target,
but is **Deferred** / scheduling-blocked pending durable authored
keyboard-color persistence, dual brightness semantics, and a sparse
frame/topology adapter. It is not migrated or a false positive and remains in
the authoritative remaining queue.

### Source-backed bulk scheduling assessment

The previous queue was **109 A + 0 B = 109 packages / 82 tentative passes**.
Removing `sabreprocs` corrects the assessment baseline to **108 A + 0 B = 108
packages**. Source-backed batch assessment replaces unproved tentative grouping
with **98 conservative scheduling passes**: ten proven two-package groups
(20 packages) plus 88 standalone target slots. This is the overall corrected
assessment baseline, including G01–G09, not 98 implementation-ready remaining
passes. After G01–G09, **90 A + 0 B = 90 Lighting targets / 89 pending scheduling
passes** remain: G10's two packages / one pass plus the same 88 standalone slots.
Nine completed two-package groups remove 18 packages and nine passes:
**108 - 18 = 90 targets; 98 - 9 = 89 passes**.
**54 standalone packages still require focused audits**.
Historical structural-scan totals remain **137 marked / 131 strong / 6 weak**.

| Group | Packages | Status |
|---|---|---|
| G01 | `darkcorergbproWU` + `darkcorergbproseWU` | Completed — `438a7d45` |
| G02 | `ironclawWU` + `ironclawSEWU` | Completed — `a20d8968` |
| G03 | `scimitarWU` + `scimitarSEWU` | Completed — `70d8b746` |
| G04 | `virtuosoWU` + `virtuosoSEWU` | Completed — `9c27548a` |
| G05 | `scufenvisionproW` + `scufenvisionproV2W` | Completed — `ed892867` |
| G06 | `darkcorergbproW` + `darkcorergbproseW` | Completed — `53c19c88` |
| G07 | `ironclawW` + `ironclawSEW` | Completed — `2a26bdc7` |
| G08 | `scimitarW` + `scimitarSEW` | Completed — `f43c4c13` |
| G09 | `virtuosoW` + `virtuosoSEW` | Completed — `08261567` |
| G10 | `k65rgb` + `k65rgbRF` | Proven bulk candidate; pending |

`darkcorergbseWU` and `k70coretklW` remain **NEEDS FOCUSED AUDIT** because each
publishes `gradient` without a proven corresponding output dispatch. Neither
audit gate is resolved by G01–G09 or this reconciliation. K60 RGB PRO remains
**Deferred A**, within the standalone target slots. Proven grouping does not
claim physical validation or authorize shared hardware abstractions.

G10 `k65rgb` + `k65rgbRF` is the only remaining original two-package group;
it is a proven bulk candidate, **pending**, not complete. After G10, work proceeds
into the standalone/focused-audit queue. Source-backed migration, automated
tests, and inert preview are acceptable for hardware not physically owned;
**no physical hardware validation was performed for G02–G09**. Complete canonical
Lighting migration remains required before legacy Lighting removal.

## Architectural signature groups

These architectural signature groups are discovery aids only. Matching
signatures, ProductType, family, or name do not establish batching. W/WU pairs
require transport/lifecycle equivalence and generally remain separate; no
remaining W/WU counterpart pair is strict-equivalence. Same-transport model
revisions are the proven bulk pattern. Prefer package-local adapters over new
shared hardware abstractions. The source-backed scheduling groups above are
separate from these historical discovery populations.

### zoned + cluster + openrgb-target + special:mouse

Count: **35**

`darkcorergbproW`, `darkcorergbproWU`, `darkcorergbproseW`, `darkcorergbproseWU`, `darkstarW`, `darkstarWU`, `glaivergb`, `glaivergbpro`, `harpoonW`, `harpoonWU`, `ironclaw`, `ironclawSEW`, `ironclawSEWU`, `ironclawW`, `ironclawWU`, `m55rgbpro`, `m65prorgb`, `m65rgbelite`, `m65rgbultra`, `m65rgbultraW`, `m65rgbultraWU`, `nightsabreW`, `nightsabreWU`, `nightswordrgb`, `sabrergbpro`, `sabrergbproW`, `sabrergbproWU`, `scimitar`, `scimitarSEW`, `scimitarSEWU`, `scimitarW`, `scimitarWU`, `scimitarprorgb`, `scimitarrgb`, `scimitarrgbelite`

G01–G03 and G06–G08 are Migrated and excluded from pending scheduling;
the 35-package signature count remains the discovery population. W/WU
counterparts completed separate source-backed passes, not an automatic
transport-equivalence migration.

### single-profile + cluster + special:keyboard

Count: **27**

`clipperpromini60`, `k100`, `k100airW`, `k100airWU`, `k57rgbW`, `k65plusW`, `k65plusWU`, `k65pm`, `k65rm`, `k70core`, `k70coretkl`, `k70coretklWU`, `k70max`, `k70pmW`, `k70pmWU`, `k70pro`, `k70protkl`, `k70rgbtklcs`, `k95platinum`, `makr75W`, `makr75WU`, `vanguard96`, `vanguard96W`, `vanguard96WU`, `vanguard96pro`, `vanguard99airW`, `vanguard99airWU`

### zoned

Count: **17**

`hs80maxW`, `hs80rgb`, `hs80rgbW`, `hs80rgbWU`, `scufenvisionproV2W`, `scufenvisionproV2WU`, `scufenvisionproW`, `scufenvisionproWU`, `virtuosoSEW`, `virtuosoSEWU`, `virtuosoW`, `virtuosoWU`, `virtuosomaxW`, `virtuosorgbXTW`, `virtuosorgbXTWU`, `voidV2W`, `voideliteW`

### single-profile + special:keyboard

Count: **18**

`k55`, `k55core`, `k55coretkl`, `k55pro`, `k55proXT`, `k57rgbWU`, `k60rgbpro`, `k65rgb`, `k65rgbRF`, `k68rgb`, `k70coretklW`, `k70lux`, `k70luxrgb`, `k70mk2`, `k70rgbRF`, `k95`, `k95platinumXT`, `strafergbmk2`

### zoned + special:mouse

Count: **6**

`darkcorergbseW`, `darkcorergbseWU`, `harpoonrgbpro`, `katarpro`, `katarproxt`, `m75`

The six-package signature count above records the discovery population, not the
remaining queue. Wired `m75` (`749f781f`) and `katarpro` (`17631e14`) are completed
standalone migrations and excluded from remaining scheduling. `katarpro` was
one standalone planned pass; no shared pass with `katarproxt` was documented.
The other four packages remain pending; the signature alone does not authorize
batching them.

### weak-marker only

Count: **6**

`motherboard`, `nexus`, `psudongle`, `psuhid`, `slipstream`, `xeneonedge`

### multi-channel

Count: **3**

`hydro`, `lncore`, `lnpro`

### multi-channel + cluster + openrgb-target

Count: **2**

`cpro`, `lt100`

### multi-channel + override + cluster + openrgb-target + special:probe-temperature

Count: **2**

`ccxt`, `cduo`

### multi-channel + per-LED + override + cluster + openrgb-target + special:led,liquid-temperature

Count: **2**

`cone`, `elite`

### Dormant/orphan RGB metadata with DPI/Sniper indicator output

Count: **1** (audited non-target; excluded from migration scheduling)

`sabreprocs` — **Not a lighting target | dormant/orphan RGB metadata |
DPI/Sniper indicator only**. Like the confirmed `m55`/`m55W` false positives,
it has no `RGBProfile` or `BrightnessSlider` authority, selectable `rgbModes`
catalogue, canonical-style effect selection, `SchedulerBrightness` percentage
Lighting authority, or general renderer / `rgb.New`. `LEDChannels=1`,
`ChangeableLedChannels=1`, and `ZoneAmount=0`. `setDeviceColor` selects the active
DPI-stage color; `SniperMode` substitutes the Sniper DPI color, and that RGB
triplet is written directly to the one physical LED. `GetRgbProfiles`,
`loadRgb`, `saveRgbProfile`, and gradient-edit helpers are dormant/orphan
metadata helpers, not proof of general Lighting capability: they ultimately
return to the DPI/Sniper indicator output path.

### dormant/inert metadata

Count: **8** (audited non-targets; excluded from migration scheduling)

`katarproW`, `m55`, `m55W`, `m75AirW`, `m75AirWU`, `nautilusLcd`, `sabrev2proW`, `sabrev2proWU`

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

Count: **1** (completed; excluded from remaining migration scheduling)

`xc7` — standalone canonical migration completed in `2e948d43`; no remaining
tentative pass.
