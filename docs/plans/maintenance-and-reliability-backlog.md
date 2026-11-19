# LumenForge Maintenance and Reliability Backlog

## Purpose and working principles

This document tracks cautious maintenance, reliability, testing, and targeted safety work. It is not a broad architectural roadmap, and its entries are not automatically release blockers.

LumenForge is a local, alpha-stage RGB and fan controller maintained by one developer. Evaluate maintenance and audit findings proportionally to that purpose, maturity, threat model, and actual usage. A technically possible edge case is not automatically a bug that must be fixed; rare or theoretical conditions should normally be recorded rather than scheduled.

- Prioritize realistic security vulnerabilities, hardware-safety risks, ordinary user-data loss, crashes, hangs, or regressions during plausible normal use, and reproducible alpha-tester reports.
- Prefer small fixes whose complexity is proportionate to the problem.
- Prefer small, inspection-first branches tied to concrete bugs or features.
- Do not refactor stable hardware-control code merely to reduce duplication.
- Add deterministic RGB output tests before broad RGB rendering changes.
- Treat repeated device code as potentially device-specific until tests and hardware evidence show otherwise.
- Introduce narrow helpers only when the relevant persistence or device code is already changing.
- Do not create generalized migration infrastructure without a real migration.
- Do not introduce broad rewrites, enterprise-grade transaction systems, generalized abstractions, or large maintenance burdens solely for contrived concurrency, filesystem, power-loss, or rollback scenarios.
- Weigh regression risk and ongoing maintenance cost against the benefit of a proposed fix.
- Give significant weight to real hardware validation and alpha-testing evidence.
- Documented findings may remain observations without becoming release blockers or planned work.

## Completed and validated

- Local-only networking.
- Localhost API and browser-origin protection.
- Immutable application and mutable runtime-state separation.
- User and system service path resolution.
- Installer hardening and rollback.
- Real user-service installation, upgrade, reboot, udev, memory, OpenRGB, cooling, RGB, and runtime-state validation.
- Runtime-path separation merged through PR #6 and installed from merge commit `72defaa7b313d5dc11b535fc404edfb6d70bfdac`.
- Post-audit reliability cleanup merged through commit `77b8fa961a309389b48854bea5004a4a43b81497`:
  - Restored the complete tracked-Go formatting baseline.
  - Made mutable LCD uploads transactional.
  - Serialized LCD upload transactions so an older failed upload cannot roll back a newer successful upload.
  - Added focused LCD tests for replacement, rollback, cleanup, cache and live-state preservation, and concurrent transactions.
  - Ensured metadata files are closed on decode failures in `cpro`, `ccxt`, `lnpro`, and `lsh`.
  - Validated with the full Go test suite, Go vet, focused race tests, frontend tests, installer checks, a successful build, and installation and normal operation on real hardware.
- Device-profile actions now publish independent Switch, Save, and Delete
  capabilities, with the shared request dispatch failing closed when the
  corresponding mutation is unavailable.
- Generic option-color saves serialize in-flight mutations and restore the last
  confirmed color after failure; a failed color can then be selected and saved
  again.
- Successful Control Dial mode changes refresh the page before selected-option
  color editing can continue, preventing a stale rendered option target from
  receiving a color mutation.
- Commander CORE shutdown lifecycle hardened: background refresh, speed, queue,
  RGB, and LCD workers synchronously stop before the final hardware-mode write
  and HID close; LCD animation waits are interruptible, preventing the HID/cgo
  shutdown race and `free(): invalid pointer` abort.
- The repository-wide modern device-workspace migration inventory is closed for
  active supported families. This is a completed presentation milestone, not
  canonical Lighting migration; deferred localization, helper-text, and
  compatibility retirement remain post-native-lighting-migration maintenance
  work.
- Native Lighting migration uses a strict source-contract check: persisted
  state, user mutation, renderer/direct output resolver, and device-owned
  physical write boundary are required, with lifecycle/reconnect proof where
  applicable. RGB-shaped metadata alone is not a target. The completed master
  audit removed Sabre V2 W/WU, Nautilus LCD, M75 AIR W/WU, and Katar Pro W
  false positives. M75 Air W/WU remain canonical Lighting non-targets: their
  DPI/Sniper indicator behavior is not a complete general Lighting contract.
  Structural sibling groups remain discovery candidates only; W/WU
  packages need focused equivalence proof before a shared implementation pass.
  M75 W/WU subsequently completed one focused canonical sibling pass
  (`90f1d10e`, *Migrate M75 wireless lighting*): canonical selected effect and
  desired Brightness, 19 existing software-rendered effects, and the
  device-authored Bottom/Logo `mouse` mode retain their device-owned
  `ZoneColors` persistence/output mapping. They have no RGB Cluster or native
  OpenRGB Integration support. Automated/source-backed validation is complete;
  no physical M75 hardware validation is recorded. Hydro then completed the
  constrained fixed-static canonical proof in `395b23b9` (*Migrate Hydro static
  lighting*): canonical fixed `static` identity, editable Static color, and
  canonical desired Brightness; no effect-selection mutation, Speed, authored
  zones, RGB Cluster, or native OpenRGB Integration; transient scheduler
  darkness; and the retained direct existing HID configuration boundary.
  Automated/source-backed validation is complete; no physical Hydro hardware
  validation is recorded.
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
  in `2e948d43` (*Migrate XC7 lighting*), separately from its modern workspace,
  Display, and telemetry milestone. Canonical presentation/state authority adapts
  existing XC7 profile persistence without a second RGB database; the 31-LED
  renderer/output and HID/LCD/temperature transport remain intact. Its exact
  23-effect catalogue includes source-backed `liquid-temperature` and CPU/GPU
  temperature; existing `MinTemp` / `MaxTemp` settings remain supported and
  internal `custom` is excluded.
  Desired `BrightnessSlider` and the `GlobalBrightness` guard are retained;
  scheduler/user RGB-off darkness is transient and profile switches clear stale
  RGB-off. Offline/stopped/missing-runtime mutations fail closed. Attachment
  failure preserves Display/telemetry, and legacy fallback remains eligible
  until a usable canonical snapshot resolves. Its canonical preview is inert.
  Focused tests/race checks passed and CodeRabbit reported no new findings;
  no physical hardware validation was performed.
  Historical proof (superseded by shared-store cutover `ca2c5954` for desired
  Lighting authority): wired KATAR PRO (`src/devices/katarpro`) completed its
  standalone canonical Lighting migration in `17631e14` (*Migrate KATAR PRO lighting*), separately from
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
  K60 RGB PRO (`k60rgbpro`) remains a complete-contract **A** Lighting target,
  but is **Deferred** / scheduling-blocked pending durable authored
  keyboard-color persistence, dual brightness semantics, and a sparse
  frame/topology adapter. It is not migrated or a false positive and remains in
  the authoritative remaining queue.
  Wired M75 removes one standalone A package/pass from the previous
  **114 A + 0 B = 114 packages / 86 tentative passes**. Detailed classification
  remains in the migration matrix. Focused source-contract audits subsequently
  classify `m55` and `m55W` as strong-marker false positives: **Not a lighting
  target | dormant/inert Lighting metadata | DPI/Sniper indicator only**. Their
  previous `single-profile` group represented one tentative migration pass;
  removing both A packages left **111 A + 0 B = 111 packages / 84 tentative
  passes**. XC7 then removes one standalone A package/pass: its matrix group
  contains only `xc7`, leaving **110 A + 0 B = 110 packages / 83 tentative
  passes**. Wired KATAR PRO then removes one standalone A package/pass in
  `17631e14`; its discovery signature group does not document a shared pass with
  `katarproxt`. The pre-reconciliation queue was
  **109 A + 0 B = 109 packages / 82 tentative passes**. Historical
  structural-scan totals are unchanged.

The previous queue was **109 A + 0 B = 109 packages / 82 tentative passes**.
Removing `sabreprocs` corrects the assessment baseline to **108 A + 0 B = 108
packages**. Source-backed batch assessment replaces unproved tentative grouping
with **98 conservative scheduling passes**: ten proven two-package groups
(20 packages) plus 88 standalone target slots. This is the overall corrected
assessment baseline, including G01–G10, not 98 implementation-ready remaining
passes. G01–G10 are complete: ten two-package groups leave **88 targets / 88
passes**. Fourteen subsequent standalone migrations complete **34 / 108 targets**
(20 grouped + 14 standalone), leaving the authoritative current queue at
**74 Lighting targets / 74 scheduling passes**. The baseline remains
**108 targets / 98 passes**; package fingerprints do not recompute it.
The earlier assessment recorded **54 standalone packages requiring focused
audits**; that historical assessment is not a refreshed current audit count.
Historical structural-scan totals remain **137 marked / 131 strong / 6 weak**.
The matrix separately flags a pre-existing one-row discrepancy between this
baseline accounting and its pending package rows (74 Legacy + deferred K60).
These progress figures preserve the supplied baseline, not a reconciled row tally.

G01–G10 are complete; package contracts and commit history are recorded in the
[native migration matrix](native-lighting-migration-matrix.md). The grouped
preview follow-up is closed, including shared keyboard spacing and K65 DE Enter
geometry (`e3f07005`); no browser or physical hardware validation is implied.
Fourteen standalone completions bring progress to **34 / 108 targets complete**,
with **74 targets / 74 passes remaining**. Migration continues
one independently audited target at a time; canonical parity is incomplete and
legacy `/rgb` removal still requires parity/cutover plus proof of no consumers.

G01 completed the source-proven same-transport Dark Core PRO bulk migration:
`darkcorergbproWU` + `darkcorergbproseWU` in `438a7d45` (*Migrate Dark Core PRO
lighting*), bringing the completed native set to sixteen packages. Package-local
adapters expose the same 20 effects and eight authored zones documented in the
migration matrix. Desired `BrightnessSlider`, transient scheduler darkness and
RGB-off, persist-before-publication, active-profile refresh, defensive copies,
Cluster/OpenRGB ownership, inert previews, and usable-snapshot legacy
fallback/suppression are preserved. Both packages normalize nil legacy
`BrightnessSlider` to 100 during profile switching. Per the migration closeout,
CodeRabbit reviewed all nine files; its minor inactive-profile switching issue
was fixed, and requested package/server/race validation passed. No physical
hardware validation was performed.

`sabreprocs` is **Not a lighting target | dormant/orphan RGB metadata |
DPI/Sniper indicator only**, analogous to `m55`/`m55W`. It has no selected-effect,
Brightness, catalogue, scheduler percentage, or general renderer authority.
Its one physical LED receives the active DPI-stage or Sniper color directly;
legacy RGB profile and gradient-edit helpers do not establish Lighting capability.

Architectural signatures are discovery aids only; ProductType, family, or name
does not establish batching. W/WU pairs require transport/lifecycle equivalence
and generally remain separate; no remaining W/WU counterpart pair is
strict-equivalence. Same-transport model revisions are the proven bulk pattern,
with package-local adapters preferred over new shared hardware abstractions.
`darkcorergbseWU`'s Gradient gate is **resolved** by `cbf0829b`: the canonical
catalogue is `mouse` / `static`; nonfunctional Gradient advertisement was retired,
with historical Gradient JSON untouched. Stored metadata is not proof of support.
`k70coretklW` remains **NEEDS FOCUSED AUDIT** for advertised Gradient with unresolved
output dispatch. K60 RGB PRO remains **Deferred A** for authored keyboard mode
durability/ownership, dual brightness systems, and sparse topology/frame mapping.

### Reusable canonical Lighting reliability rules

- Validate complete proposed desired state, persist successfully, then publish
  and restart/output. Persistence errors must be observable; failure must not
  partially publish state or emit output. Never fall back to legacy writes.
- Keep canonical worker cancellation/ownership separate from legacy `d.activeRgb`
  and unbuffered `Exit` channels. Legacy rendering remains fallback-only when
  canonical attachment is unusable, not a parallel desired-state authority.
- Reject stale asynchronous Native/Cluster/OpenRGB frames using owner generation
  or session checks. Refresh external ownership registration safely across profile
  switches and invalidate stale callbacks/frames on ownership changes.
- Transform defensive copies of authored colors/settings so repeated Static
  scaling or transient overlays cannot corrupt stored state.
- Distinguish persisted user RGB-off preference from runtime-only scheduler
  darkness. An independently persisted preference is not duplicate canonical
  effect/settings/desired-brightness authority; disconnect/replay semantics need
  source-backed handling.

### Required presentation cleanup for confirmed non-Lighting targets

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

## Observations to revisit only if reproduced

### OpenRGB profile persistence failure handling

Some OpenRGB profile persistence paths may handle unusual filesystem failures imperfectly. Normal effect changes, cluster-mode changes, profile persistence, service restart, and state restoration have been manually validated successfully.

A large transactional redesign was investigated but intentionally not retained because its complexity and regression risk were disproportionate to the rare theoretical failure modes. Do not redesign this subsystem pre-emptively. Revisit it only when alpha testing produces a reproducible failure under realistic conditions, or when related persistence code must change for a concrete feature or bug.

## Conditional reliability investigations

### `activeRgb` ownership and goroutine lifecycle

Similar `activeRgb` pointer ownership and goroutine lifecycle patterns exist across many device modules. No normal-use crash, shutdown failure, or race has been demonstrated.

Do not begin a repository-wide or representative-device investigation solely because the repeated pattern exists. Revisit it only after a reproducible crash, an actual race report, a shutdown or effect-lifecycle failure, or an alpha-tester report tied to this area. Any future work must begin with one physically available device and focused lifecycle tests, not a broad sweep.

### M65 RGB Ultra wireless follow-up

Receiver `m65rgbultraW` (`8d48d18d`) and direct-USB `m65rgbultraWU`
(`4e0676d9`) are canonical Lighting complete, independently. The findings below
are deferred decision-support items, not migration blockers or hardware-confirmed
bugs. Classification describes current source evidence; no new tests or hardware
operations were run for this documentation reconciliation. Revisit concrete
failures with focused fixes before considering broad persistence/lifecycle work.

**WU — direct USB**

| Classification | Remaining finding and action |
|---|---|
| Unverified concurrency risk | Full-profile legacy saves (`saveDeviceProfile` and callers) may lose overlapping changes. Canonical preference/DPI overlap was corrected; review unrelated legacy save paths only when persistence work or a reproducer warrants it. Coordinate with **Thin profile and path helpers** / **Atomic persistence primitives** below rather than opening duplicate general persistence work. |
| Source-backed overlapping operation; UI reachability unverified | `SetLightingEffect` persists the Native effect, then separately resumes RGB-off for the same profile. An explicit off request between those steps can be cleared and unexpectedly relight the device. Establish ordinary UI reachability before prioritizing command-order changes. |
| Source-backed transient limitation | `changeLightingDeviceProfile` publishes/captures new indicator state and refreshes external registrations before `toggleDPI` completes programming. External Logo output can briefly carry the new indicator with old hardware DPI. Check physical impact during profile switching before changing sequencing. |
| Confirmed source-level test-fixture defect; developer-test maintenance | `TestWUReviewPollingTransition` sends reports into a bounded channel with a blocking send. If reports saturate it, the output stub can strand fixture cleanup. Bound or cancel the fixture send when maintaining this test; this is not a demonstrated device runtime hang. |
| Optional test coverage | Rainbow polling already asserts a fresh worker, resolved Rainbow effect, saturated Logo spectrum, and progression. `TestWUSecondReviewPollingReadFailureRecovery` still checks restored output headers rather than the exact restored Logo payload; strengthen that targeted assertion. Do not reopen the resolved Rainbow/header-only finding. |
| Inherited HID limitation | `transferLocked` uses blocking `dev.Read`; cancellation cannot interrupt a read already issued. Worker cancellation is separate, but in-flight transport completion still depends on HID. Revisit only with an observed shutdown/recovery failure and focused transport evidence. |
| Source-backed telemetry limitation | CPU/GPU temperature effect dispatch consumes `CpuTemp` / `GpuTemp`, but the audited WU package does not populate those samples. Complete sampling in a separate focused change before claiming live temperature-effect accuracy. |
| Optional cleanup | Renderer Gradient sorting/re-densification repeats the dense canonical projection; catalogue validation is repeated, and lifecycle state is complex. Simplify only during related work with output/ownership coverage; none is a demonstrated runtime defect by itself. |

Source anchors: WU `lighting.go` (mutations/preferences), `lighting_output.go`
(profile switching, rendering, recovery), `m65rgbultraWU.go` (legacy saves and
HID), and `lighting_review_test.go` (polling fixtures/assertions).

**W — receiver-owned peripheral**

| Classification | Remaining finding and action |
|---|---|
| Source-backed readiness difference; physical availability impact unverified | `setupLightingConnection` requires successful software-mode/endpoint setup and assignment replay, unlike permissive legacy heartbeat-connected behavior. Optional preferences remain best-effort; invalid stored mappings can be skipped and repair-persistence deferral can retain connection availability. Check real wake failures before changing readiness. |
| Hardware validation needed | Exercise receiver sleep/wake and reconnect under Native rendering, external ownership, and input writes using the checklist below. Source/session checks do not establish physical recovery. |
| Source-backed storage-policy limitation | `changeCanonicalDeviceProfile` prepares assignments before activation and returns failure on read/decode/repair-write errors. Damaged assignment storage can therefore prevent profile activation. Reconnect already retains custom bindings for deferred repair and skips invalid replay shapes; do not report that fixed availability behavior as an open bug. Review activation/repair policy with the existing profile/path and persistence maintenance topics, preserving stored custom data. |
| Optional simplification | Aggregate input revisions and assignment/profile replay coupling in `lighting_input.go` / `lighting_connection.go` increase state complexity. Revisit with a concrete maintenance obstacle or reproducer. |
| Profiling needed | `validateLightingSource` repeatedly validates the full catalogue on frame-resolution paths. Measure cost before optimizing; no performance regression is established. |
| Optional cleanup | `RebindReceiver` currently has test callers but no production caller found. W-local GPU sampling in `lighting_temperature.go` duplicates telemetry logic. Revisit removal/sharing only during related lifecycle/telemetry work, preserving bounded sampling and cancellation. |

### M65 wireless physical alpha validation

**NOT PHYSICALLY VERIFIED — every unchecked item below remains outstanding.**
These checks are separate from source-level migration completion and existing
automated fixtures. Record actual device/firmware and results when performed;
do not infer a pass from source review or previews.

**W — receiver-owned peripheral**

- [ ] Cold start and warm wake.
- [ ] Receiver sleep/wake during Native rendering.
- [ ] Receiver disconnect/reconnect under Native and external ownership.
- [ ] Input commands during disconnect and subsequent replay/recovery.
- [ ] Sniper/DPI programming and indicator correctness.
- [ ] Scheduler darkness and persisted RGB-off, including wake restoration.
- [ ] External Logo bytes preserved during local indicator updates.
- [ ] Profile switching and assignment replay, including damaged-storage policy.
- [ ] Setup/write failure recovery without stale-session output.

**WU — direct USB**

- [ ] Cold USB startup and bounded endpoint recovery.
- [ ] Effect changes and desired/effective Brightness.
- [ ] Persisted RGB-off and transient scheduler darkness/restoration.
- [ ] Sparse/tied Gradient playback.
- [ ] DPI/Sniper programming and indicator colors; profile-switch timing.
- [ ] Polling-rate transitions with input continuity and Lighting restoration.
- [ ] Cluster output and Native/Cluster/OpenRGB ownership transitions.
- [ ] OpenRGB controller-slot stability through polling recovery/failure.
- [ ] Shutdown and subsequent recovery, including transport-read behavior.

## Deferred maintenance — trigger required

These subjects are not scheduled merely because they are listed. Begin one only when activated by a concrete feature, a reproducible bug, an alpha-testing report, a real schema migration, or a demonstrated maintenance obstacle, while preserving its stated prerequisites and safety guidance.

### Deterministic RGB output tests

Before broad RGB rendering work, add byte-exact tests for representative rainbow, wave, static, gradient, temperature-based, inverted, AIO, and LCD-masked output. Use fixed start times and deterministic random seeds. These tests are the prerequisite for broad output assembly, timing, dispatch, or buffer-finalization changes.

### Thin profile and path helpers

When profile persistence is already changing, consider small helpers for category paths, typed JSON loading, existing save behavior, and directory creation. Preserve filenames, directory layout, fallbacks, error behavior, and overwrite semantics; test with temporary directories.

### Atomic persistence primitives

When related persistence code changes, consider narrow temporary-file, sync, and atomic-replacement primitives. Preserve existing schemas and successful behavior, and add failure-path tests before adopting a helper elsewhere.

### Effect timing cleanup

Only after deterministic timing tests exist, a small helper may centralize elapsed time, speed factors, or default-speed handling for effects already being modified. Preserve animation speed and direction exactly; do not perform a repository-wide conversion.

### Cluster dispatch cleanup

Simplify cluster effect dispatch only when a concrete change makes the existing switch materially difficult to maintain. Keep special cases explicit, preserve arguments and side effects, and compare outputs before and after.

### Careful OpenRGB handler sharing

Share OpenRGB handler code only where behavior is demonstrably compatible. Persistence, error handling, live versus durable state, speed overrides, rollback, controller requirements, and asynchronous lifecycles differ; prefer narrow helpers over a generic wrapper.

### Versioned migrations

Create versioned migration infrastructure only for a real incompatible profile or configuration schema change. Any such work must include backups, clear version detection, idempotence, temporary-directory integration tests, and protection against overwriting user data.

### Unsupported SysV init script

Retire the unsupported SysV init script or modernize it only when that installation path has a confirmed user and can be validated. Do not treat it as equivalent to the supported user and system service paths.

### Post-native-migration compatibility retirement

After every native lighting family has completed its individual migration and
consumer tracing, retire retained legacy compatibility deliberately: global
`/rgb` routes and mutation helpers, stale Dashboard settings/model fields,
legacy temperature-bar preference plumbing, dead `ShowLabels` plumbing,
obsolete theme compatibility selectors, and unused scripts/templates/routes.
Do not remove tests that protect retained compatibility behavior before the
behavior itself is retired. Localization and helper-text consistency work that
was deferred during individual workspace migrations belongs in the same
post-migration maintenance pass, not in hardware migrations.

## Current project priorities

1. Complete native canonical Lighting migration one source-confirmed family at
   a time.
2. External Source Registry redesign.
3. Backup restore hardening.
4. Legacy RGB compatibility retirement after native Lighting migration and
   consumer tracing.

## Active security work tracked separately

The following remain active because they address realistic trust-boundary and untrusted-input concerns rather than theoretical maintenance edge cases:

- External Source Registry redesign.
- Backup restore hardening.

Their implementation and completion status are tracked separately.
