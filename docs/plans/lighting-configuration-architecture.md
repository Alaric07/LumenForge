# Lighting Configuration Architecture

## 1. Status and scope

This document defines the target architecture for LumenForge software-lighting
configuration. It covers:

- OpenRGB-imported device software lighting;
- RGB Cluster software lighting;
- eventual migration of native RGB-capable device families away from the
  standalone RGB editor.

This is a deliberate clean break for alpha software. Compatibility with old
lighting customization data is not required. The first production work remains
focused on OpenRGB-imported devices and RGB Cluster. Native device families
migrate separately and only after their hardware-specific behavior and required
controls are understood; they are not part of one broad migration milestone.

The architecture describes the intended final system. Current global RGB
editing, RGB Override, heterogeneous Static zone colors, and
base/override/effective presentation are temporary legacy structures, not
features to reproduce under new names.

## 2. Product goals

- Lighting settings live where output ownership lives.
- Independent devices are configured on Device Lighting.
- Cluster output is configured on RGB Cluster.
- Users do not edit a global effect library.
- Users do not interact with a separate override layer.
- The renderer and presentation snapshot resolve the same settings.
- Future OpenRGB imports automatically use the same Lighting workspace.
- Adding a software effect requires one canonical description of its
  configurable inputs.
- Fixed and generated-palette effects quietly omit palette controls.
- The backend has one deterministic source of resolved effect settings.

## 3. Final ownership model

There are exactly two active software-lighting scopes.

### Independent device

The device scope owns:

- the selected effect;
- device Brightness;
- complete per-effect custom settings.

Device Lighting is the authoritative editing surface while the device owns its
output.

### RGB Cluster

The cluster scope owns:

- the selected effect;
- cluster Brightness;
- complete per-effect custom settings.

RGB Cluster is the authoritative editing surface for cluster output. Cluster
settings are ordinary cluster-owned settings, not overrides.

When a device is cluster-controlled:

- cluster lighting is authoritative;
- local device lighting controls are disabled or unavailable;
- the device's own settings remain stored but inactive;
- leaving the cluster restores the device's own resolved settings.

## 4. Hidden immutable defaults

Shipped effect defaults remain internal. `database/rgb.json` is the expected
authoritative shipped source unless implementation work proves that a smaller
source is safer. Defaults are validated while loading into an immutable
repository, and callers receive defensive copies rather than shared mutable
maps, slices, or pointers.

Defaults are neither editable nor presented in the UI. When a target has no
customization for an effect, resolution returns the hidden default. Reset
deletes the target/effect customization and resolves the hidden default again.

One target must never be able to mutate:

- another target's settings;
- cluster defaults;
- future device defaults;
- the shipped/default repository.

Tests must prove isolation at every mutable level, including Gradient stops.

## 5. Complete customization records

The replacement uses complete custom effect definitions, not sparse
field-level inheritance.

A device/effect or cluster/effect has either one complete custom record or no
custom record. On the first genuine edit, the complete current resolved effect
definition becomes that target's customization. Later edits replace fields in
the complete record. Reset deletes the record.

This gives deterministic behavior when shipped defaults change:

- uncustomized effects and reset effects use the new shipped default;
- already-customized effects remain stable until reset.

The persisted value needs a small, explicit shape capable of representing:

- Speed;
- one color;
- Start and End colors;
- Low, Middle, and High temperature points;
- a Celsius threshold for each temperature point;
- ordered Gradient stops;
- each Gradient stop's position, color, and relative intensity.

Optional variants may indicate which complete shape applies, but they must not
mean "inherit this missing field." Validation requires exactly the complete
settings variant allowed by the canonical descriptor. Gradient data should use
an ordered stop structure rather than depending on mutable map indexing. A
small schema version is appropriate for rejecting unknown future formats; it
does not imply support for migrating retired alpha data.

## 6. Canonical resolution

The central operation is conceptually:

    ResolveEffectSettings(scope, targetID, effectID)

It returns the complete target customization when present. Otherwise it
returns a defensive copy of the hidden default. The returned value is complete
and renderer-ready in either case.

Both of these consumers use that same resolved value:

- the renderer and output path;
- the UI presentation snapshot.

The HTTP server must not recreate precedence or independently inspect
persistence structures. Target packages retain lifecycle and output ownership,
while one lighting-settings package owns default lookup, target customization
storage, validation, defensive copying, and resolution.

Selected effect and Brightness remain target state. Brightness is deliberately
separate from an effect customization.

## 7. Renderer-driven controls

The selected effect and its canonical descriptor determine which
effect-specific controls exist.

### Off

Off has no effect-specific controls.

### Single-color effects

Single-color effects show one native color input and one exact editable hex
field. Current examples are:

- Static;
- Rotator;
- Visor.

### Two-color effects

Two-color effects show:

- Start;
- End.

Each role uses a native color input and exact editable hex field.

### Temperature effects

Temperature effects show:

- Low color and threshold;
- Middle color and threshold;
- High color and threshold.

Current examples are CPU Temperature and GPU Temperature. Their renderers
consume three per-color Celsius thresholds. They do not consume the generic
profile `MinTemp` and `MaxTemp` fields, so those generic fields do not belong in
this editor.

The canonical renderer contract was completed in `3f840533`. CPU Temperature
uses Low, Middle, and High thresholds of 20, 50, and 95 Celsius in the shipped
defaults; GPU Temperature uses 20, 50, and 80 Celsius. Thresholds are finite and
strictly ordered as Low < Middle < High. Exact thresholds and interpolation are
deterministic, while malformed or incomplete three-point data resolves to a
usable Low point or otherwise black. Semantic point roles are never sorted or
reassigned. The legacy two-point path and temperature Brightness behavior were
not changed.

### Gradient

Gradient shows:

- ordered Gradient stops;
- stop position;
- stop color;
- stop intensity;
- Speed when supported.

The completed renderer contract keeps stop intensity relative and separate
from owning-scope Brightness. The future editor exposes those distinct values;
it is not implemented by the renderer milestone.

### Fixed and generated-palette effects

Fixed and generated-palette effects show no palette controls. They show Speed
only when the canonical descriptor says Speed is supported. Current generated
examples include:

- Aurora;
- Color Warp;
- Cyberpunk Glitch;
- Flame;
- Nebula;
- Rainbow;
- Pastel Rainbow;
- Spiral Rainbow;
- Pastel Spiral Rainbow;
- Tokyo Night;
- Water Color.

No empty palette section or explanatory "generated palette" message is shown.

## 8. Final page structure

Device Lighting and RGB Cluster use the same conceptual order:

1. Effect
2. Brightness
3. Speed, when supported
4. Applicable color, temperature, or Gradient controls
5. Reset selected effect to defaults, only when customized

The final UI removes:

- Effective stored palette;
- Stored precedence;
- Device RGB definition;
- Local OpenRGB override;
- Effective configuration;
- capability-description cards;
- override facts;
- generated-palette explanations;
- duplicated base/override/effective readouts;
- links to the standalone RGB editor.

The final UI retains:

- effect identity and icon where useful;
- exact editable hexadecimal values;
- native color inputs;
- accessible labels, keyboard behavior, and visible focus;
- semantic theme support;
- responsive layout;
- concise pending, success, and failure status;
- one nearby cluster-ownership explanation when local controls are disabled.

## 9. Brightness ownership

Brightness is scope-wide state and is separate from effect definitions.

Final behavior is:

- an independent device applies device Brightness exactly once;
- RGB Cluster applies cluster Brightness exactly once;
- individual device Brightness remains stored but inactive while the device is
  cluster-controlled;
- member device callbacks do not apply device Brightness to an already-scaled
  cluster frame;
- effect Reset never resets Brightness.

The Gradient renderer contract was completed in `fe8e2462`. Stop selection and
stop-aware color and relative-intensity interpolation occur first, after which
owning-scope Brightness scales the completed output exactly once. Maximum
owning Brightness preserves valid prior output, zero produces black, and neither
stop intensity nor owning Brightness is duplicated. Rendering also preserves
caller-owned Gradient data and resolves the circular last-to-first segment on
both sides of the cycle boundary.

The imported RGB Cluster member Brightness defect was corrected in
`267effb0`. Cluster Brightness now scales the completed aggregate output exactly
once, and OpenRGB-imported member callbacks transmit their assigned bytes
without reapplying stored local device Brightness. Independent OpenRGB output
continues to apply local device Brightness once. Cluster dispatch also copies
each member segment before concurrent callback execution so callbacks cannot
mutate the aggregate frame.

## 10. Static behavior

Static has one color per owning scope:

- one color per independent device;
- one color per RGB Cluster;
- every LED in that owning scope receives the same color.

Zones remain available for names, LED counts, topology, offsets, and output
addressing. They do not independently own Static colors. Heterogeneous
per-zone Static colors are retired, and no migration or alternate editing mode
is planned.

`ZoneColors`, `lastColor`, and `RGBOverride` are not final Static desired-state
sources. Old Static customization may be discarded during the clean break.

`af25b031` completed this ownership cutover for independent OpenRGB-imported
devices. Static now resolves one complete single-color setting through the
canonical resolver, fills the complete device frame uniformly, and applies
device Brightness exactly once. Zones remain topology and addressing metadata
only. Heterogeneous `ZoneColors`, `lastColor`, Static RGB Override precedence,
legacy profile colors, and target-local RGB files no longer determine
independent Static output. The legacy OpenRGB Static mutation path and its
obsolete template controls were retired without migration or fallback.

`22f8117c` completed the resolved Device Lighting control cutover for
independent OpenRGB-imported devices. Base/override/effective presentation was
replaced by canonical resolved control data, the workspace was reduced to
renderer-supported controls, and strict single-color editing plus selected
effect Reset were added. Reset removes only the selected effect customization,
preserves the selected effect and device Brightness, and avoids renderer
interruption when no customization exists.

`ad4b0340` corrected two issues found during manual hardware validation:
uncustomized Static now renders its editor from the resolved shipped default,
and the Reset control becomes visible immediately after the first successful
effect-specific customization rather than requiring a page refresh. Brightness
changes remain independent and do not reveal effect Reset. Focused, repeated,
race, full-repository, JavaScript, and CodeRabbit validation passed. Manual
testing confirmed Static color editing, animated Speed customization, Reset,
cluster ownership, Brightness independence, and persistence across restart.

`20e6d472` added complete Start/End editing for independent OpenRGB-imported
devices whose canonical descriptor uses a two-color palette. Presentation
resolves both colors from the same canonical settings used by rendering, and
each mutation persists one complete Start/End pair rather than sparse
role-specific overrides. Existing resolved Speed, selected effect, and device
Brightness remain unchanged.

The editor initializes from shipped defaults when uncustomized, supports exact
hexadecimal and native color inputs, and uses the existing selected-effect
Reset semantics. Cluster-owned controls remain visible but disabled without
exposing local Reset. Focused automated tests and CodeRabbit review passed.
Manual hardware testing confirmed independent Start and End changes, complete
pair persistence, Reset, Speed and Brightness preservation, cluster ownership,
and restoration across restart.

`33ea040c` added complete Low/Middle/High temperature editing for independent
OpenRGB-imported devices whose canonical descriptor uses the three-point
temperature contract. Presentation resolves all three colors and Celsius
thresholds from the same canonical settings used by rendering, and every
mutation persists one complete semantic Low/Middle/High set rather than sparse
point-specific changes.

The editor initializes from shipped resolved defaults, supports native and exact
hexadecimal color inputs plus finite Celsius thresholds, and preserves strict
Low < Middle < High ordering without sorting or reassigning semantic roles.
Generic `MinTemp` and `MaxTemp` fields remain outside the editor. Existing
selected-effect Reset semantics restore the complete shipped temperature
settings while preserving the selected effect and device Brightness.
Cluster-owned controls remain visible but disabled without local Reset.

Automated tests and CodeRabbit review passed. Manual hardware testing confirmed
the distinct CPU and GPU shipped thresholds, independent color and threshold
changes, complete persistence across restart, ordering rejection, Reset,
Brightness preservation, cluster ownership, and exclusion from non-temperature
palette editors.

The currently rendered Palette capability value remains a temporary diagnostic,
not part of the final user-facing design. It should be removed after the
remaining renderer-driven editors make the selected effect's applicable inputs
self-evident.

## 11. OpenRGB imports

OpenRGB importing remains separate from lighting customization. The import
store owns information such as:

- controller identity;
- imported serial;
- external serial;
- product, vendor, and location metadata;
- zone names;
- zone LED counts;
- disabled state;
- topology needed for addressing.

It does not own the selected effect, Brightness, colors, Speed, Gradient data,
or override state.

A newly imported device should:

1. appear in the Devices workspace;
2. initially resolve hidden defaults;
3. create target customizations only after the user changes an effect setting.

The existing OpenRGB import store may be carried into a clean installation
only while its exact schema and identity semantics remain compatible.

## 12. Clean-break policy

No compatibility code is required for old lighting state. Do not implement or
plan automatic lighting migration, dual reads, dual writes, override
conversion, zone-color conversion, global RGB-profile conversion, or a startup
migration framework.

A redesigned alpha release may instruct users to rename or remove old mutable
data and configure lighting again. The current audit classifies the following
as safe or conditionally safe backup candidates:

- `config.json`;
- `display.json`;
- `database/audio.json`;
- `database/temperatures/`;
- `database/macros/`;
- `database/key-assignments/`;
- `database/lcd/`;
- `external-sources.json` for the same host and trust policy;
- `database/openrgbimport-zones.json` only while its exact schema remains
  compatible;
- `dashboard.json` only while its exact schema remains compatible.

Expected old lighting data to discard includes:

- `database/profiles/`;
- `database/rgb/`;
- `database/led/`;
- old or experimental `database/lighting/`;
- broad all-in-one backup archives for this transition;
- `database/scheduler.json` while RGB scheduling remains mixed with unrelated
  scheduler state.

Final release documentation must revalidate this list against the completed
implementation. No automatic backup or restoration is part of this design.

## 13. Systems to retire

The final architecture removes:

- user-facing and production `RGBOverride`;
- OpenRGB override persistence and precedence;
- per-device mutable RGB copies under `database/rgb/<serial>.json`;
- the cluster mutable RGB copy under `database/rgb/cluster.json`;
- heterogeneous OpenRGB Static zone-color persistence;
- `lastColor` as desired state;
- base/override/effective snapshots and presentation;
- the standalone `/rgb` editor;
- global RGB editor mutation endpoints;
- the legacy OpenRGB Static color endpoint;
- duplicated capability lists after descriptor adoption.

Code serving native-device families may remain temporarily until those
specific families receive replacement controls. The initial OpenRGB work does
not promise immediate deletion of shared native-device infrastructure.

## 14. Native-device boundary

The standalone RGB editor currently serves native-device families as well as
OpenRGB-imported devices and RGB Cluster. Therefore:

- OpenRGB may cut over independently;
- RGB Cluster may cut over independently;
- native families migrate one at a time;
- `/rgb` is removed only when every still-supported target it serves has
  equivalent renderer-consumed controls and Reset behavior;
- no permanent global editor remains after parity.

Initial OpenRGB work must not broadly redesign native hardware protocols,
packet formats, lifecycle behavior, or device-owned firmware effects.

## 15. Modern mutation principles

Future mutations require:

- exact target identity;
- the exact expected selected effect for effect-specific changes;
- descriptor and supported-catalogue validation;
- scope validation;
- lifecycle validation;
- an RGB Cluster ownership guard for local device changes;
- strict JSON decoding;
- bounded request size;
- unknown-field rejection;
- checked persistence;
- in-memory rollback on persistence failure;
- persistence before output;
- bounded first-output waiting;
- no duplicate renderer workers;
- generic browser-facing errors;
- retention of persisted desired state after an output failure.

The intended mutation categories are conceptually:

- select effect;
- set Brightness;
- set Speed;
- set descriptor-valid colors;
- set Gradient data;
- reset customization.

Exact route names are intentionally not frozen as a permanent external API
contract in this architecture document.

## 16. Reset semantics

Reset is scoped to the selected target and selected effect. It:

- deletes only that target/effect customization;
- preserves the selected effect;
- preserves Brightness;
- resolves and reapplies the hidden default;
- restores in-memory state and produces no output if persistence fails;
- reports an output failure while retaining the persisted reset state;
- starts at most one replacement worker;
- is hidden when no customization exists.

For fixed and generated-palette effects, Reset can still remove customized
Speed or another genuine renderer-consumed setting.

## 17. Implementation sequence

1. Define and test Low/Middle/High temperature threshold semantics. Completed
   in `3f840533`.
2. Make owning-scope Brightness authoritative for Gradient while preserving
   per-stop intensity as a relative property. Completed in `fe8e2462`.
3. Correct imported RGB Cluster member double scaling in a focused commit.
   Completed in `267effb0`.
4. Add lighting settings types, an immutable default repository,
   defensive-copy validation, dedicated stores, a resolver, and path tests.
   Completed in `4f0a38a6`.
5. Add dedicated OpenRGB device-lighting persistence and atomically cut
   OpenRGB effect selection, Brightness, Speed, rendering, restart, and
   reconnect to the resolver. Completed in `c91a970c`.
6. Make OpenRGB Static uniform and remove `ZoneColors` and `lastColor` as
   desired-state sources together with the legacy OpenRGB color path. Completed
   in `af25b031`.
7. Replace OpenRGB snapshots with resolved control data, simplify Device
   Lighting, and add single-color editing and Reset. Completed in `22f8117c`,
   with manual-test corrections in `ad4b0340`.
8. Add two-color editing. Completed in `20e6d472`.
9. Add temperature color and threshold editing. Completed in `33ea040c`.
10. Add Gradient editing.
11. Cut RGB Cluster persistence and rendering to the resolver while preserving
    membership and device order separately.
12. Modernize RGB Cluster with the shared descriptor-driven controls.
13. Delete OpenRGB override code and legacy imported-device lighting controls.
14. Migrate native target families separately.
15. Remove `/rgb` after every remaining consumer has parity.
16. Remove global mutations, target-local RGB copies, remaining override
    infrastructure, duplicate capability adapters, and obsolete CSS and
    documentation.
17. Add final clean-install, backup, and release documentation.

Every milestone must build and have focused tests. No milestone may
permanently dual-read or dual-write old and new lighting state. A legacy
subsystem may temporarily remain only for a target not yet cut over, and every
temporary subsystem needs a named deletion condition.

## 18. Non-goals

- Migrating old lighting data.
- Preserving `RGBOverride`.
- Preserving heterogeneous Static zone colors.
- Exposing or editing hidden defaults.
- Resetting Brightness with effect Reset.
- Inventing palette controls for fixed or generated effects.
- Migrating every native-device family in one change.
- Changing OpenRGB identity or import semantics.
- Changing hardware protocols.
- Redesigning cooling, LCD, macros, input assignments, dashboard, or unrelated
  profiles.
- Adding a general database migration framework.
- Automatically backing up or restoring data.
