# UI Redesign and Maintainability Roadmap

The LumenForge UI redesign is not merely a visual reskin or a complete
application rewrite. It modernizes and simplifies the dashboard, makes
capabilities truthful and discoverable, reduces unnecessary duplicated
bookkeeping, and makes effects, devices, controls, and themes easier to add.
It preserves hardware-specific behavior where duplication is intentional and
advances through small, reviewed milestones rather than a giant rewrite.

The target software-lighting ownership, persistence, resolution, control, and
clean-break decisions are defined in the
[Lighting Configuration Architecture](lighting-configuration-architecture.md).

## Status conventions

- `[x]` Completed
- `[~]` In progress, or foundation created but not integrated
- `[ ]` Planned
- `[!]` Known issue or decision required

Unless otherwise stated, completion means the work is committed and pushed to
`ui/redesign`. This roadmap has no speculative dates or delivery promises.

## 1. Guiding principles

- Build a modern, desktop-class hardware-control experience for Linux.
- Preserve the three-region architecture:
  - far-left global navigation;
  - middle device list;
  - main selected-device workspace.
- Put global telemetry on the main Dashboard instead of repeating it on every
  device and settings page.
- Show device-specific telemetry only when a device truthfully reports it.
- Let RGB Cluster controls own cluster-wide lighting.
- Let Device Lighting controls own local per-device lighting.
- When RGB Cluster owns output, keep local mutation controls disabled and show
  one nearby ownership explanation.
- Keep Brightness separate from effect customizations and apply it exactly once
  in the owning scope.
- Resolve renderer output and presentation snapshots from the same canonical
  target settings.
- Keep shipped effect defaults hidden and immutable.
- Define semantic theme token values in themes and style components once.
- Give shared effect metadata one canonical source.
- Keep software-rendered effects distinct from similarly named native firmware
  effects.
- Keep hardware protocols and device-specific behavior owned by device
  packages.
- Remove duplicated bookkeeping where data can safely be derived.
- Preserve intentional backend-specific implementations.
- Prefer narrow migrations with focused tests and review.
- Retain a target's legacy route only until that target reaches replacement
  parity and its named deletion condition is satisfied.

## 2. Completed milestones

### Devices workspace

- [x] `70872dc6` — Add isolated Devices workspace
- [x] `00ab0666` — Add device selection workspace
- [x] `1897a739` — Add OpenRGB workspace overview

These commits established the modern three-region Devices workspace, added
server-rendered device selection, and introduced an overview for imported
controllers. Existing legacy device pages remain available; not every legacy
device control has migrated.


### Native device control workspaces

- [x] `d9ea1dd6` — Add Scimitar Elite DPI workspace
- [x] `afadd8f6` — Sync active DPI and sniper state
- [x] `e9904362` — Add Scimitar Elite key assignments workspace
- [x] `08341f0a` — Add toggle mode for Sniper assignments
- [x] `99a1bcd4` — Add RGB Cluster effect cycling assignment
- [x] `7447672f` — Add shared device performance workspace

These milestones established the first modern native-device control workspace
beyond Lighting. Scimitar RGB Elite now exposes its existing DPI configuration,
live active-stage and Sniper state, physical button assignments, and performance
settings through the shared Devices workspace.

The visible native-device workspace is organized around Overview, Lighting,
Performance, and Key Assignments. The Performance tab currently retains the
internal `view=dpi` route while combining the existing DPI editor with
capability-driven performance controls.

K95 Platinum now provides Overview, Lighting, and Keyboard workspaces. Its
Keyboard workspace contains Keyboard Settings, Key Lockouts, and Color & Key
Assignments. The nested Color & Assignment Preset is a keyboard-specific preset
concept, distinct from the full Device Profile shown on Overview. Full Device
Profiles use the existing device backend to represent the complete device
configuration: K95 Platinum and Scimitar RGB Elite expose them on Overview.

The shared Performance presentation contract treats Polling Rate, Angle
Snapping, and Lift Height as optional capabilities. The shared frontend uses
device-generic `/api/devices/performance/*` routes while existing device
packages remain authoritative for mutation, validation, persistence, and
hardware behavior. Scimitar RGB Elite is the first hardware-validated
Performance provider.

RGB Cluster effect cycling is also available as a Scimitar RGB Elite physical
button assignment without changing the existing Profile Switch assignment.
Cluster effect changes made from either the physical assignment or system tray
are reflected by the RGB Cluster page through lightweight canonical status
synchronization.

### Mutation and persistence hardening

- [x] `e44a2bb3` — Harden OpenRGB lighting mutations
- [x] `e1770a8c` — Fix OpenRGB static override output
- [x] `e116554d` — Preserve OpenRGB temperature middle color
- [x] `1fad3c99` — Harden OpenRGB RGB override persistence
- [x] `426f262b` — Copy OpenRGB override into user profiles

These changes hardened LumenForge lighting mutations, override output, and
profile persistence for OpenRGB-imported devices. OpenRGB is the hardware
communication backend; it is not the source of LumenForge software animations.
They describe historical hardening of the current alpha architecture. The
clean-break architecture intentionally retires the override layer rather than
carrying it forward.

### Capability and read-only workspace

- [x] `bfc0c751` — Add OpenRGB lighting capability snapshot
- [x] `c02d1eae` — Add read-only OpenRGB lighting workspace

The capability snapshot distinguishes palette kinds, cluster ownership,
supported effects, current configuration, and effective output. The read-only
Lighting workspace presents that information without changing native-device
implementations. Its current comparison-oriented presentation is an
intermediate milestone and is not the final control model.

### Theme system

- [x] `1be764c4` — Expand semantic theme system

The theme milestone established:

- a semantic `--lf-*` token contract;
- built-in `default`, `catppuccin-macchiato`, `cyberpunk`, `dracula`, and
  `tokyonight` themes;
- the default theme as a fallback layer before the selected theme;
- components that consume semantic tokens instead of theme-specific component
  styling;
- exact user-configured RGB swatches that are not theme-adjusted.

### Interactive effect selection

- [x] `01b80d4a` — Add OpenRGB effect selector

This milestone added alphabetically sorted, user-facing supported-effect
labels while preserving stable IDs, an explicit `Off` fallback label, a
protected mutation request, bounded timeout handling, and restoration after
failure. Cluster-owned selection is disabled with an adjacent explanation, the
RGB Cluster link works, and Overview and Lighting have larger semantic tabs.
Native devices remain on their existing path. These are LumenForge effects
targeting OpenRGB-imported devices, not animations supplied by OpenRGB.

### Renderer safety

- [x] `7b09eb1a` — Harden Flickering and Visor renderers

The Flickering low-speed random-bound panic was fixed while keeping its random
target reachable. Visor now has a truthful one-LED static Start-color fallback.
Focused renderer contracts cover 1, 2, 4, and 8 LEDs.

### Canonical software-effect descriptor registry

- [x] `b7fb8bb5` — Add software effect descriptor registry

The registry defines exactly 35 generic LumenForge software effects with typed
scope, palette, color usage, persistent-speed support, sensor requirement,
topology, minimum LEDs, and icon identity. Lookup returns descriptor values and
the list API returns a defensive copy in deterministic order.
All 35 are currently classified as compatible with both an individual device
LED buffer and RGB Cluster's combined buffer.
The registry is committed, but production consumers have not migrated to it yet.

## 3. Confirmed effect-system findings

### Generic effects

The audit confirmed 35 generic LumenForge software effects that operate against
an ordered LED buffer and are suitable for both:

- a compatible individual device buffer;
- RGB Cluster's combined buffer.

No generic software renderer was confirmed to require multiple devices,
controller boundaries, or cluster coordinates.

### Device-specific effects

These seven effects remain outside the generic registry:

- `colorwave`
- `led`
- `liquid-temperature`
- `probe-temperature`
- `rainbowwave`
- `tlk`
- `tlr`

Keyboard firmware effects may depend on firmware, key events, direction, or key
matrix behavior. Liquid- and probe-temperature effects depend on device-local
sensors. Per-LED profiles depend on a device-local indexed layout. Those
requirements remain owned by their device implementations.

### OpenRGB-imported-device catalogue

All 35 generic software effects are now selectable for individual
OpenRGB-imported devices. The catalogue is derived from canonical descriptor
Device scope, explicit dispatch is covered by consistency tests, and rendering
checks current-device eligibility immediately before dispatch. Unsupported
preserved IDs still render the conservative Static fallback. This completed
catalogue work does not settle the replacement persistence architecture or the
renderer-driven editing controls.

### Existing icons

- Every implemented stable effect ID has a matching SVG at
  `static/img/icons/rgb/{stable-id}.svg`.
- Existing SVGs hard-code `#5599FF`.
- External image loading prevents automatic `currentColor` inheritance.
- Theme-aware recoloring is implemented through CSS masks in the modern
  OpenRGB-imported-device Lighting workspace (`401a132e`); it is no longer an
  unresolved icon-strategy decision.

## 4. Clean-break lighting architecture

The detailed target contract is recorded in the
[Lighting Configuration Architecture](lighting-configuration-architecture.md).
The implementation is a deliberate clean break: old custom lighting data may
be discarded, and no migration, dual-read, dual-write, RGB Override conversion,
zone-color conversion, or global RGB-profile conversion is planned.

### Architecture foundation

- [x] Establish the canonical generic software-effect descriptor registry.
- [x] Migrate generic software-effect capability lookup to canonical
  descriptors (`287c0f89`).
- [x] Migrate generic software-effect persistent-speed support to canonical
  descriptors (`b895cb75`).
- [x] Derive the OpenRGB-imported-device catalogue from descriptor Device scope
  (`e3df7bcd`).
- [x] Use canonical descriptor icon identity in modern presentation
  (`401a132e`).
- [x] Define and test Low/Middle/High temperature threshold semantics
  (`3f840533`).
- [x] Make owning-scope Brightness authoritative for Gradient while preserving
  per-stop intensity as a relative property (`fe8e2462`).
- [x] Correct imported RGB Cluster member double scaling in a focused commit
  (`267effb0`).
- [x] Add immutable hidden defaults with defensive-copy tests
  (`4f0a38a6`).
- [x] Add complete effect-settings types and dedicated device/cluster stores
  (`4f0a38a6`).
- [x] Add the canonical target/effect resolver and path tests
  (`4f0a38a6`).
- [x] Add dedicated OpenRGB device-lighting persistence
  (`c91a970c`).
- [x] Derive the RGB Cluster catalogue from descriptor scope (`09f14f87`).
- [ ] Remove duplicated metadata only after migrated consumers pass parity
  tests.
- [ ] Keep renderer dispatch explicit until a proven replacement preserves
  output and lifecycle behavior.

`3f840533` made canonical CPU/GPU Low/Middle/High threshold semantics explicit,
with deterministic malformed-data fallback and unchanged valid shipped
behavior. This is renderer and metadata foundation work, not the temperature
editor UI.

`fe8e2462` made owning Brightness scale completed Gradient output exactly once
while keeping stop intensity relative and maximum-Brightness output compatible.
It also corrected the pre-existing pre-first-stop circular wrap defect. This is
renderer foundation work, not the Gradient editor UI.

`267effb0` corrected imported RGB Cluster member Brightness double scaling.
Cluster Brightness now scales aggregate output once, imported member callbacks
do not reapply stored local device Brightness, and independent OpenRGB output
continues applying local Brightness once. Member segments are copied before
concurrent callback dispatch to preserve aggregate-frame ownership.

`4f0a38a6` added the dormant lighting-settings foundation: immutable shipped
defaults with deep defensive copies, complete descriptor-validated effect
settings, dedicated independent-device and RGB Cluster customization stores,
and the canonical target/effect resolver. Reset is represented by deleting one
target/effect customization, and the dedicated stores neither read nor write
legacy RGB profile paths. No runtime lighting consumer was cut over in that
milestone.

`c91a970c` atomically cut OpenRGB-imported independent-device lighting to the
new ownership model. Selected effect and device Brightness now use dedicated
device state, complete per-effect settings use the device customization store,
and rendering, presentation, restart, and reconnect resolve through the
canonical resolver. Existing effect, Brightness, and conditional Speed
interactions were preserved. Legacy target-local RGB files and user-profile
lighting fields are no longer authoritative for those values. At that
milestone, heterogeneous Static zone state, `lastColor`, the legacy Static
endpoint, Static RGB Override behavior, and base/effective presentation
compatibility remained deliberately isolated for later milestones.

`af25b031` completed uniform canonical Static ownership for independent
OpenRGB-imported devices. Static now resolves one device-wide color through the
canonical resolver, preserves zones only for topology and addressing, and
applies device Brightness exactly once. `ZoneColors`, `lastColor`, Static RGB
Override precedence, legacy profile colors, and target-local RGB files are
inert for independent Static output. The legacy Static mutation route and its
obsolete template controls were removed without migration or fallback. Focused,
repeated, race, and full-repository validation passed; CodeRabbit reported no
findings, and a fresh-state manual test confirmed shipped-default cyan output,
canonical effect and Speed persistence for three devices, and successful
restoration after restart.

`22f8117c` replaced the intermediate base/override/effective snapshot with
canonical resolved control data, simplified Device Lighting to genuine
renderer-supported controls, and added strict single-color editing and selected
effect Reset. Reset deletes only the selected device/effect customization,
preserves the selected effect and Brightness, reapplies the hidden default, and
treats an already-uncustomized effect as a true no-op.

`ad4b0340` corrected two issues discovered during manual hardware validation.
An uncustomized Static effect now renders its color editor from the resolved
shipped default, and a successful first Speed customization reveals Reset
immediately without requiring a browser refresh. Brightness changes remain
independent and do not reveal Reset. Automated focused, repeated, race,
full-repository, JavaScript, and CodeRabbit validation passed. Manual testing
confirmed Static color editing, Reset, animated Speed customization, cluster
ownership, Brightness independence, and persistence across restart.

`20e6d472` added the descriptor-driven two-color Start/End editor for
independent OpenRGB-imported devices. Both resolved colors are presented from
the canonical settings used by rendering, and changing either role persists
one complete Start/End customization while preserving resolved Speed, selected
effect, and device Brightness. The editor uses paired native color and exact
hexadecimal inputs, strict bounded mutation handling, shared status feedback,
and the existing effect Reset behavior.

Uncustomized effects display shipped resolved defaults. Cluster-owned controls
remain disabled without local Reset, while other palette kinds do not render
the two-color editor. Focused automated tests and CodeRabbit review passed.
Manual hardware testing confirmed independent Start and End changes, complete
pair persistence, Reset, Speed and Brightness preservation, cluster ownership,
and restoration across restart.

`33ea040c` added descriptor-driven Low/Middle/High temperature color and Celsius
threshold editing for independent OpenRGB-imported devices. All three semantic
points are presented from canonical resolved settings, and changing any color
or threshold persists one complete Low/Middle/High customization while
preserving the selected effect and device Brightness.

The editor uses native color and exact hexadecimal inputs together with Celsius
numeric controls, rejects malformed or non-finite state and invalid threshold
ordering, and never exposes legacy `MinTemp` or `MaxTemp` controls. CPU and GPU
Temperature resolve their distinct shipped defaults, while cluster-owned
controls remain disabled without local Reset and non-temperature palette kinds
do not render the editor. Automated tests and CodeRabbit review passed. Manual
hardware testing confirmed editing, ordering validation, persistence, Reset,
Brightness preservation, cluster ownership, and restoration across restart.

`ed9d5016` added descriptor-driven ordered Gradient editing for independent
OpenRGB-imported devices. The workspace presents complete resolved stops with
native and exact hexadecimal color inputs, normalized Position, relative
Intensity, Add and Remove controls, and an explicit complete Save operation.
Draft changes remain local until Save; valid saves stable-sort by Position while
preserving equal-position order and persist one complete Gradient customization
without changing the selected effect, resolved Speed, or device Brightness.

The backend requires complete already ordered Gradient data and does not sort
caller payloads. Cluster-owned controls remain disabled without local Reset,
while non-Gradient palette kinds do not render the editor. Hardware validation
found that the historical shipped positions 0, 0.33, 0.66, and 1 produced an
abrupt circular seam because the final and first colors occupied the same cycle
boundary. The shipped default was corrected to positions 0, 0.25, 0.5, and
0.75, all at relative intensity 1, which preserves the existing renderer's
smooth circular last-to-first interpolation. Automated focused, repeated, race,
full-repository, JavaScript, build, and CodeRabbit validation passed, and manual
hardware testing confirmed smooth looping and Gradient editing behavior.

`5a58140f` removed the temporary user-facing Palette capability readout from
Device Lighting now that each renderer-driven palette shape has its own
applicable controls. The obsolete Palette metric, configuration-rail capability
fact, and dedicated metric styling were removed while internal `PaletteKind`
control gating remains intact. Focused server tests, the full repository test
suite, and CodeRabbit review passed.

### OpenRGB Device Lighting cutover

- [x] Add the modern protected effect selector (`01b80d4a`).
- [x] Add the theme-aware Brightness control (`e46d19ec`).
- [x] Add the conditional persisted Speed control (`45d1a6c9`).
- [x] Adapt effect selection, Brightness, and Speed from current persistence to
  the canonical resolver without redesigning their established interactions
  (`c91a970c`).
- [x] Cut rendering, restart, and reconnect atomically to the resolver
  (`c91a970c`).
- [x] Make Static one color per device; retain zones only for topology,
  addressing, names, and LED counts (`af25b031`).
- [x] Retire heterogeneous OpenRGB Static zone colors, `lastColor`, Static RGB
  Override precedence, and the legacy Static mutation path without migration
  (`af25b031`).
- [x] Replace base/override/effective snapshots with resolved control data,
  simplify Device Lighting to renderer-supported controls, and add the
  Static/single-color editor and Reset (`22f8117c`, corrected in `ad4b0340`).
- [x] Add the two-color Start/End editor (`20e6d472`).
- [x] Add Low/Middle/High temperature color and threshold editing (`33ea040c`).
- [x] Add the ordered Gradient editor (`ed9d5016`).
- [x] Remove the temporary user-facing Palette capability readout after the
  renderer-driven editors make palette shape self-evident (`5a58140f`).
- [ ] Delete OpenRGB RGB Override and legacy imported-device lighting paths
  after modern parity.

Static is one color per independent device. There is no zone-color migration,
alternate heterogeneous Static mode, or user-facing override layer. Fixed and
generated-palette effects show only genuine renderer-supported controls and no
palette placeholder or explanatory message.

Reset deletes only the selected device/effect customization, preserves the
selected effect and device Brightness, resolves the hidden default, and is
shown only while that customization exists.

`fd2b3ab4` separated device and RGB Cluster resolver ownership, and `af08ec39`
completed the Cluster persistence/rendering cutover. Cluster target state,
complete effect customizations, and ordered device layout now have separate
canonical stores. Cluster Brightness is applied once to the aggregate frame,
membership remains device-owned, Static is uniform across the owning Cluster
scope, and legacy Cluster RGB/profile files are no longer lighting authorities.

The cutover also made scheduler lights-out transient rather than persisted
Brightness state, moved Dashboard lighting presentation to the canonical Cluster
snapshot, and hardened single-worker ownership and restart behavior. Hardware
validation using copied real user state with native and OpenRGB-imported
members confirmed persisted `static` / Brightness 60 restoration across restart.
A tray-startup hotfix discovered during that validation was removed because it
incorrectly rewrote the loaded Cluster effect to Rainbow.

`09f14f87` added the dedicated canonical Cluster lighting control API,
`f719e5f4` completed the shared descriptor-driven RGB Cluster workspace, and
`235941d0` added Cluster-scoped effect Reset. Effect, Brightness, conditional
Speed, single-color, two-color, Temperature, Gradient, status, and Reset now
use the same modern interaction model as Device Lighting while retaining
target-specific persistence and mutation contracts.

`0ccc2d3f` removed the remaining Cluster compatibility/global RGB model
surface. RGB Cluster no longer exposes mutable legacy RGB profile projections
or legacy profile mutation methods, and legacy global RGB helpers explicitly
skip Cluster while continuing to serve native-device consumers. The system
tray Cluster submenu now derives canonical Cluster-scoped descriptors and
selects through `SetLightingEffect`.

The legacy `/rgb` editor remains for targets that still require it, but RGB
Cluster is no longer listed there and is no longer mutable through its generic
legacy color/profile routes. Scheduler lights-out, member order, canonical
state, canonical effect customizations, and the transient renderer adapter
remain intact.

### RGB Cluster cutover

- [x] Add dedicated cluster settings persistence and canonical resolution
  (`fd2b3ab4`, `af08ec39`).
- [x] Make cluster Brightness authoritative and apply it exactly once
  (`af08ec39`).
- [x] Preserve cluster membership and device order outside effect settings
  (`af08ec39`).
- [x] Use the shared descriptor-driven Effect, Brightness, Speed, color,
  temperature, Gradient, and status components (`09f14f87`, `f719e5f4`).
- [x] Add cluster-scoped Reset with the same deletion semantics (`235941d0`).
- [x] Make cluster Static one color across the complete cluster output
  (`af08ec39`).
- [x] Remove cluster dependence on the mutable global RGB profile model after
  renderer, persistence, restart, and UI parity (`0ccc2d3f`).

### Native-device migration

- [x] Complete the first native Device Lighting migration set. Scimitar Pro RGB
  established the shared canonical proof; Scimitar RGB Elite, MM800, and K95
  Platinum now use canonical Device Lighting without participating in retained
  legacy `/rgb` lighting persistence or mutation paths.

The retained global `/rgb` path uses `eligibleForLegacyGlobalRGB()` as a
temporary migration bridge: unmigrated native families remain eligible, while
OpenRGB-imported and Cluster devices remain excluded. A native canonical
Lighting provider is excluded only when its `LightingSnapshot()` and runtime are
usable; a provider whose canonical runtime did not attach falls back to retained
`/rgb`. That runtime boundary does not require immediate deletion of every
package-local legacy-looking helper. Such helpers are not authoritative lighting
state for a migrated package and cannot make it participate in the global path.
The bridge and its compatibility machinery remain until the final `/rgb` cleanup
after every legitimate native consumer has migrated.

- [x] Extract the shared independent-device lighting runtime and move Scimitar
  Pro selected effect to canonical state (`d833da87`, with canonical-read fixes
  in `4aa688a2`).
- [x] Move Scimitar Pro desired Brightness to canonical independent-device
  state (`35b7bb3f`).
- [x] Make Scimitar Pro scheduler/lights-out Brightness transient, preserve the
  latest desired Brightness, and immediately replay externally owned ordinary
  zones with the device-owned DPI indicator recomputed from effective
  Brightness (`5c7763c7`).
- [x] Move Scimitar Pro complete per-effect customization to canonical
  `EffectSettings` persistence (`90a4b003`).
- [x] Move Scimitar Pro ordered Gradient add/delete mutations to canonical
  settings (`b1fb390f`).
- [x] Synthesize retained Scimitar `/rgb` presentation from canonical resolved
  settings and remove legacy `d.Rgb` from effect-presentation authority
  (`4e37de16`).
- [x] Add immutable canonical native Lighting snapshots and expose migrated
  native devices in Devices -> Lighting (`d58cb007`, `f6c1cdc2`).
- [x] Add the reusable native Device Lighting mutation contract for effect,
  Brightness, Speed, supported palette settings, and selected-effect Reset
  (`da1e0597`).
- [x] Make native Device Lighting interactive without routing native devices
  through `/api/openrgbimport/*` (`0aadedb2`).
- [x] Migrate Scimitar RGB Elite selected effect, Brightness, renderer settings,
  and its device-authored `mouse` mode to canonical native Lighting
  (`2c758431`, `ce890f75`).
- [x] Migrate MM800 selected effect, Brightness, renderer settings, and its
  device-authored `mousepad` mode to canonical native Lighting
  (`8b9eebe9`, `bfc4c5cd`, `ce890f75`).
- [x] Migrate K95 Platinum selected effect, Brightness, generic effect settings,
  renderer input, and restart behavior to canonical native Lighting while
  retaining its keyboard protocol, per-key state, keyboard presets, RGB Cluster
  behavior, and lifecycle as device-owned concerns.
- [x] Add the shared authored-zone presentation and mutation contract used by
  device-owned native modes. It supports persistent multi-selection, clear
  selection, selected-zones and all-zones mutations, strict validation,
  device-owned persistence, and suppression of local ordinary-zone output
  while RGB Cluster or retained OpenRGB integration owns the device
  (`ce890f75`).
- [x] Keep authored-zone presentation metadata semantic rather than blindly
  exposing legacy profile layout/group fields. MM800 presents its 15 zones in
  stable numeric order and does not expose meaningless legacy row grouping or
  overlapping legacy geometry; Scimitar RGB Elite presents Front, Scroll,
  Side, and Logo while keeping DPI outside the authored-zone editor
  (`ce890f75`).
- [x] Remove Scimitar Pro, Scimitar RGB Elite, MM800, and K95 Platinum from
  retained legacy `/rgb` editing and startup compatibility after eliminating
  their remaining legacy RGB persistence and mutation dependencies.
- [x] Complete Commander Core XT's controller proof: modern Overview, Lighting,
  and Cooling; full Device Profile; shared Cooling presentation; canonical
  multi-channel Lighting; backend-owned 3-Pin RGB Port topology; and
  `probe-temperature` (`040159e9`, `54e32705`, `6f221ab2`, `8715fa62`).
- [x] Complete Commander CORE's separate controller proof without assuming XT
  hardware parity: modern Overview, Lighting, and Cooling; full Device Profile;
  pump/AIO RGB and `liquid-temperature`; FreeLedPorts-based Custom RGB Device
  fallback; and capability-driven optional Display (`ef7335a6`, `09b7f6f1`,
  `c20dde8f`, `729aa3d7`). Its LCD path is automated-test validated but awaits
  supported physical LCD hardware.
- [ ] Preserve each later family's protocol, packet, topology, lifecycle,
  firmware, device-specific lighting modes, and hardware-specific output
  behavior while repeating migration one family at a time.
- [ ] Remove shared override/global-editor infrastructure only after every
  proven remaining consumer reaches parity.

### Native-device workspace expansion

- [x] Add a physically available keyboard as the second native shared-workspace
  proof. K95 Platinum completes the physical mouse-and-keyboard proof while
  reusing existing native device behavior rather than introducing replacement
  hardware logic.
- [ ] Make Performance presentation fully device-neutral where the second device
  class exposes assumptions inherited from the initial mouse implementation.
- [ ] Enrich Overview as a read-only, capability-driven device summary now that
  a physical mouse and keyboard are represented. It may show active effect,
  ownership or RGB Cluster state, active profile, polling or performance state,
  and truthful device statistics only when the selected device reports them.
- [ ] After the shared contract is proven across owned device classes, add thin
  Performance providers to other already-native devices where their existing
  implementations expose the required current state, option inventory, and
  mutation methods. Do not treat code-derived support as hardware validation.
- [ ] Treat memory as a separate upcoming family-specific migration target;
  future native families still require individual audit and parity because their
  channels, sensors, cooling, per-LED behavior, and other device-owned
  capabilities differ materially from existing proofs.
- [ ] Keep legacy native pages available until the applicable modern workspace
  reaches control parity for that device family.

### Final cleanup

- [ ] Remove `/rgb` after OpenRGB, RGB Cluster, and every still-supported native
  target it serves has renderer-consumed parity.
- [ ] Remove global RGB editor mutation endpoints after their consumer search
  is empty.
- [ ] Remove remaining mutable target-local RGB copies.
- [ ] Remove remaining RGB Override infrastructure.
- [ ] Remove duplicated capability structures after descriptor adoption.
- [ ] Remove obsolete templates, JavaScript, CSS, tests, and documentation.
- [ ] Add final clean-install, selective-backup, and release guidance.

The current backup boundary expects `config.json` to remain reusable. The
OpenRGB import store is reusable only while its exact schema and identity
semantics remain compatible. Final release documentation must revalidate the
complete backup list against the finished implementation.

## 5. Established Lighting workspace interactions

The theme-aware brightness control establishes the reusable slider pattern:

- A native accessible range input uses shared theme-aware styling, with
  Firefox and WebKit presentation driven by existing semantic tokens.
- An editable native numeric percentage input stays synchronized with the
  range. Range movement previews locally and commits one mutation on release;
  numeric entry commits on Enter, change, or blur without duplicate requests.
- Both inputs share one mutation path, timeout, in-flight lock, confirmed
  baseline, and failure-restoration path. Pending and success messages are
  transient, while failure remains until a later valid interaction.
- The persisted value survives reload and restart. RGB Cluster ownership
  disables both controls and is independently enforced by
  `Device.SetBrightness`.
- Unavailable brightness renders neither a slider nor a fabricated percentage.
- Automated, repeated, and race tests passed, along with controlled browser
  and hardware validation and CodeRabbit review.

The persisted Speed control extends that pattern for OpenRGB-imported devices:

- It appears only for the active LumenForge software effects whose canonical
  descriptor supports persistent speed. The paired semantic, theme-aware range
  and numeric controls present a `1.0` through `10.0` scale while delegating
  duration, identity, and calibrated effect mappings to the established
  `rgb-speed.js` helper.
- Exact legacy stored values remain unchanged until a genuine edit. The
  established control and interaction remain complete UI capabilities, but
  their current persistence source will be adapted to the canonical resolver
  during the OpenRGB cutover.
- Checked persistence rolls back on save failure, while an already-persisted
  desired speed remains stored if initial renderer output later fails or
  exceeds its five-second wait. Browser errors remain generic.
- RGB Cluster ownership disables the local controls and is independently
  enforced by the device mutation. The legacy categorical request remains
  compatible, but the modern persisted slider does not use that path.
- Deterministic JavaScript coverage, focused and broader Go tests, repeated and
  race tests, server template and error-redaction tests, and manual Firefox
  interaction validation passed. Three CodeRabbit review rounds ended with no
  findings across all ten milestone files.

Settled interaction pattern for future sliders:

- Pointer range movement previews locally.
- Pointer release commits one persisted mutation.
- Repeated keyboard range or numeric-arrow adjustments remain focused and
  coalesce into one mutation after 400 milliseconds of inactivity.
- Enter and blur can commit immediately without duplicate requests.
- Raw numeric text is not rewritten while the user is typing.
- Selection and caret behavior remain native.
- Formatting or normalization occurs only during initialization, explicit
  commit, restoration, success, or failure rollback.
- Keyboard-origin mutations restore focus when the user has not deliberately
  moved elsewhere. Pointer- and blur-origin commits do not steal focus.
- Pending state, timeout handling, confirmed baselines, rollback, and
  generation-safe status timers remain control-local.
- Permanent instructional text is unnecessary.
- Transient status appears only when useful.

Control design expectations:

- Use custom, modern, theme-aware presentation while preserving native
  accessibility and keyboard behavior.
- Avoid stock-looking unstyled sliders and excessive animation or gimmicks.
- Consume semantic theme tokens.
- Use hardened request handling, a bounded timeout, restoration after failure,
  and duplicate-request prevention for every mutation.

## 6. Effect catalogue migration and expansion

- [x] Use canonical descriptors for capability metadata (`287c0f89`).
- [x] Add missing explicit importer dispatch cases (`9fb3eab2`).
  Importer rendering now checks the current device's exact supported-effect
  catalogue immediately before dispatch. Unsupported preserved profile IDs
  remain stored unchanged but render the Static fallback; `spiralrainbow` and
  `pastelspiralrainbow` obey the same boundary while absent from the catalogue.
- [ ] Add focused renderer contract coverage where it is missing.
- [x] Expose all compatible LumenForge software effects to individual
  OpenRGB-imported devices (`e3df7bcd`).
  The importer catalogue is now derived from canonical descriptor Device scope,
  with defensive catalogue slices per device and all 35 generic LumenForge
  software effects selectable. Consistency tests cover explicit dispatch,
  capability and speed metadata, profiles, and render-time eligibility.
  Controlled live hardware validation passed for newly exposed effects,
  persistence, reconnect, existing effects, cluster ownership, and the legacy
  importer page.
- [ ] Verify behavior with 1, 2, 4, and 8 LEDs.
- [ ] Visually calibrate effects that are safe but limited on short buffers.
- [ ] Keep device-specific and firmware-native effect catalogues separate.
- [x] Migrate the RGB Cluster catalogue to scope-based filtering (`09f14f87`).
- [ ] Remove superseded duplicated lists only after parity tests pass.

An effect looking better in a cluster is not sufficient reason to classify it
as cluster-only.

## 7. Effect icon integration

- [x] Show the selected effect's existing icon in the Lighting workspace
  (`401a132e`).
- [x] Map icons by stable effect ID, never by display label.
- [x] Keep the native select text-only.
- [x] Retain an accessible text label.
- [x] Provide a safe generic CSS fallback for unknown or unsupported stored
  IDs.
- [x] Do not guess mappings from similar effect names.
- [x] Render the canonical SVG artwork through CSS masks, with visible color
  supplied by existing semantic theme tokens.
- [x] Validate icon presentation in every built-in theme.

Canonical icon filenames now come from software-effect descriptors, and all 35
generic effects have validated SVG mappings. Unsupported stored IDs never
create arbitrary asset URLs, while the native text-only selector and the real
Static, two-color, temperature, Gradient, Off, and generated-palette
presentations remain intact. No JavaScript, SVG asset, descriptor, catalogue,
renderer, persistence, native-device, RGB Cluster, or theme file changed. All
five built-in themes passed manual browser inspection; automated tests, race
coverage, broader validation, and CodeRabbit also passed.

## 8. Device architecture and future device support

- [ ] Make new devices expose truthful capabilities rather than requiring
  UI-specific assumptions.
- [ ] Keep native devices on their existing protocol and firmware behavior.
- [ ] Add native-device adapters to the modern Devices workspace gradually.
- [ ] Keep legacy `/device/{serial}` available until feature parity exists.
- [ ] Show device telemetry only when the device provides it.
- [ ] Use stable local device-image assets with accessible fallbacks.
- [ ] Make unsupported devices fail gracefully instead of rendering empty
  controls.
- [ ] Avoid broad hardware refactors solely to satisfy presentation code.

### Future new-device checklist

- [ ] Protocol implementation
- [ ] Stable identity and metadata
- [ ] Capability snapshot or adapter
- [ ] Persistence behavior
- [ ] Device image or icon
- [ ] Hardware-free unit tests where possible
- [ ] Controlled hardware validation
- [ ] Modern workspace integration
- [ ] Legacy fallback preservation until parity

## 9. Theme architecture and future themes

The semantic component contract already exists. Built-in themes provide token
values rather than redefining components, and new controls must use the same
contract. `default.css` remains the fallback layer. New themes must define the
complete required contract, while custom or older themes must degrade safely.

Add visual checks for selectors, tabs, sliders, color controls, ownership
notices, focus states, and narrow layouts.

### Future theme checklist

- [ ] Token file
- [ ] Complete-contract test
- [ ] Settings discovery
- [ ] Visual review
- [ ] Focus and contrast review
- [ ] No component duplication

## 10. Dashboard and application shell

- [ ] Add collapsible far-left global navigation.
- [ ] Show icon and label in the expanded state.
- [ ] Show an icon with an accessible name and tooltip in the collapsed state.
- [ ] Keep the selected state clear.
- [ ] Persist the user's preference.
- [ ] Preserve keyboard support.
- [ ] Use a smooth but restrained width transition.
- [ ] Keep the device list as a separate middle column.
- [ ] Keep responsive behavior independent from desktop collapse state.
- [ ] Put global CPU, GPU, and storage telemetry on the Dashboard.
- [ ] Avoid repeating global telemetry on every page.
- [ ] Review top-level Information and Settings pages for semantic-theme and
  shell consistency.

The collapsible sidebar is a separate milestone from lighting mutations.

## 11. Usability and accessibility

- [ ] Keep selected and inactive states clear.
- [ ] Use comfortable minimum target sizes.
- [ ] Preserve visible focus styles.
- [ ] Use semantic navigation and real links.
- [ ] Place ownership messages beside disabled controls.
- [ ] Never make an explanation depend solely on a cursor or tooltip.
- [ ] Use `aria-live` mutation feedback where appropriate.
- [ ] Use `aria-describedby` for related restriction text.
- [ ] Prevent horizontal page overflow in responsive layouts.
- [ ] Keep all controls keyboard-operable.
- [ ] Present truthful empty, unavailable, and unsupported states.
- [ ] Avoid diagnostic-console styling in primary user workflows.

## 12. Testing strategy

Expected validation layers:

- pure renderer contract tests;
- descriptor and catalogue consistency tests;
- package tests and vet;
- deterministic JavaScript tests;
- request timeout and restoration tests;
- server template and escaping tests;
- theme contract tests;
- CodeRabbit review for substantial or risk-bearing milestones when
  proportionate;
- documentation-only, copy-only, and trivial styling milestones may rely on
  local validation and maintainer review;
- controlled browser validation;
- controlled hardware validation only when behavior reaches hardware;
- no hardware requirement for metadata-only commits.

Small-buffer renderer coverage targets are 1, 2, 4, and 8 LEDs. Random effects
must use deterministic contract tests rather than probabilistic retry loops.

## 13. Duplication-removal plan

| Duplicated concern | Current sources | Canonical target | Migration status | Removal condition |
| --- | --- | --- | --- | --- |
| Effect IDs and labels | Shipped profiles, device and cluster lists, presentation fallbacks | Software-effect descriptors | `[~]` Registry created | All migrated consumers pass parity tests |
| Palette and color usage | `LightingEffectCapabilities`, profile defaults, presentation logic | Software-effect descriptors | `[~]` Registry created | Capability and presentation consumers use descriptors |
| Speed support | `HasSpeedControl`, capability metadata, controls | Software-effect descriptors | `[~]` Canonical metadata integrated and consumed by the OpenRGB-imported-device persisted Speed control; RGB Cluster and native-device migration remain incomplete | Persistence and UI parity tests pass |
| Sensor requirement | Renderer dispatch and device-specific paths | Generic descriptors where applicable | `[~]` Generic CPU/GPU metadata created | Generic consumers migrate; device-local sensors remain separate |
| Topology and scope | Renderer knowledge and target-specific lists | Software-effect descriptors | `[~]` Registry created | Catalogue filtering and parity tests pass |
| Device-target effect list | OpenRGB-imported-device allowlist and native lists | Scope-filtered generic descriptors plus native capabilities | `[~]` OpenRGB generic catalogue migrated; native lists remain | Explicit dispatch and compatibility tests exist for each target |
| RGB Cluster effect list | Cluster catalogue | Scope-filtered generic descriptors | `[x]` Migrated (`09f14f87`) | Cluster list and behavior parity tests pass |
| Icon identity | Stable asset naming and presentation paths | Software-effect descriptors | `[~]` Identity recorded | Presentation uses descriptor identity with fallback tests |
| Renderer dispatch | Importer and cluster switches | Explicit dispatch until a proven replacement exists | `[!]` Decision required | Output parity and lifecycle behavior are demonstrated |
| Default profile values | `database/rgb.json` and mutable target-local copies | Hidden immutable default repository plus complete target customizations | `[~]` OpenRGB integrated; other targets pending (`4f0a38a6`, `c91a970c`) | Resolver and defensive-copy tests pass; migrated targets no longer read local RGB copies |
| Effect-setting precedence | Device RGB definitions, RGB Override, zone colors, and `lastColor` | One target customization or hidden default | `[~]` OpenRGB animated state integrated; Static and other targets pending (`c91a970c`) | Renderer and presentation use the same resolver |
| Native firmware capabilities | Native device packages and firmware mode lists | Device-owned capability adapters | Intentional separation | Normalize only proven shared semantics |

Suitable centralization targets are IDs and labels, palette and color usage,
speed support, sensor requirement, topology, target scope, icon identity, and
generic catalogue filtering.

Renderer implementations and dispatch, hardware protocol commands, native
firmware capability lists, device-local sensor behavior, hardware-specific
default values, and firmware effects that share a semantic ID with a software
effect remain intentionally separate or only partially centralizable. This
roadmap does not promise that every duplicate will be deleted.

## 14. Recommended milestone order

1. [x] Complete and commit canonical descriptor registry.
2. [x] Add this living roadmap.
3. [x] Migrate capability metadata to descriptors (`287c0f89`).
4. [x] Migrate speed metadata to descriptors (`b895cb75`).
5. [x] Add missing importer dispatch cases (`9fb3eab2`).
6. [x] Expand compatible LumenForge software effects to individual
   OpenRGB-imported devices (`e3df7bcd`).
7. [x] Integrate selected-effect icons (`401a132e`).
8. [x] Add theme-aware brightness control (`e46d19ec`).
9. [x] Add conditional speed control (`45d1a6c9`).
10. [x] Record the clean-break lighting configuration architecture in
    [Lighting Configuration Architecture](lighting-configuration-architecture.md).
11. [x] Define and test Low/Middle/High temperature threshold semantics
    (`3f840533`).
12. [x] Make owning-scope Brightness authoritative for Gradient while
    preserving per-stop intensity as a relative property (`fe8e2462`).
13. [x] Correct imported RGB Cluster member Brightness double scaling
    (`267effb0`).
14. [x] Add immutable defaults, complete settings types, dedicated stores, and
    the canonical resolver (`4f0a38a6`).
15. [x] Cut OpenRGB effect selection, Brightness, Speed, rendering, restart,
    and reconnect to dedicated device-lighting persistence and the resolver
    (`c91a970c`).
16. [x] Make OpenRGB Static uniform and retire zone-owned Static colors and the
    legacy color path without migration (`af25b031`).
17. [x] Simplify Device Lighting presentation and add single-color editing and
    Reset (`22f8117c`, corrected in `ad4b0340`).
18. [x] Add two-color editing (`20e6d472`).
19. [x] Add temperature color and threshold editing (`33ea040c`).
20. [x] Add Gradient editing (`ed9d5016`).
21. [x] Cut RGB Cluster persistence, rendering, catalogue, and Brightness to
    the resolver and shared controls (`fd2b3ab4`, `af08ec39`, `09f14f87`,
    `f719e5f4`, `235941d0`).
22. [ ] Remove OpenRGB RGB Override and legacy imported-device lighting paths
after OpenRGB parity.
23. [~] Migrate native-device families one at a time without changing their
hardware-specific output behavior. Scimitar Pro RGB, Scimitar RGB Elite, MM800,
K95 Platinum, Commander Core XT, and Commander CORE are fully migrated to the
canonical Device Lighting model and no longer participate in legacy `/rgb`
lighting persistence or mutation paths. Remaining native families are still
pending.
24. [x] Add the generic native authored-zone presentation and mutation contract
for device-owned modes (`ce890f75`).
25. [ ] Remove `/rgb`, global mutations, remaining target-local RGB copies,
remaining override infrastructure, and duplicated metadata after every
proven consumer reaches parity.
26. [ ] Add final clean-install, selective-backup, and release guidance.
27. [ ] Continue shell, Dashboard telemetry, responsive, accessibility, and
polish milestones independently.

This order may change when implementation uncovers a prerequisite or defect.
Record any change and its reason in this document.

## 15. Known boundaries and non-goals

- This is not a full rewrite.
- Do not remove legacy native-device pages before feature parity.
- Do not replace device protocol implementations with a generic abstraction.
- Do not assume same-named firmware and software effects are identical.
- Do not claim hardware support without verification.
- Do not bypass cluster ownership from local device controls.
- Do not expose effects without explicit renderer dispatch and safety coverage.
- Do not perform a giant one-commit metadata migration.
- Do not migrate, convert, dual-read, or dual-write retired alpha lighting
  state.
- Do not retain the standalone RGB editor after all served targets have parity.
- Do not expose hidden defaults or make them user-editable.
- Do not reset Brightness when resetting an effect customization.
- Do not require every effect to look ideal at every LED count.
- Do not restore Docker as part of the UI work.
- Do not add a general migration framework for discarded lighting state.

## 16. Open decisions

- [!] Persistence location for collapsed sidebar state
- [!] Whether future renderer dispatch remains switch-based or uses registered
  function references
- [!] Degree of native-device capability normalization
- [!] Representation of indexed per-LED palettes in shared metadata
- [!] Representation of liquid and probe sensors in future capability types
- [!] Whether short-buffer visual warnings are useful or unnecessarily noisy

Resolve these only with implementation evidence.

## 17. Maintaining this roadmap

Future milestones should update this roadmap by checking completed items,
adding commit hashes where useful, recording newly discovered defects, and
noting changes to milestone order. Keep architectural decisions separate from
speculative ideas, and remove or update stale `[~]` markers promptly.
