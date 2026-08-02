# UI Redesign and Maintainability Roadmap

The LumenForge UI redesign is not merely a visual reskin or a complete
application rewrite. It modernizes and simplifies the dashboard, makes
capabilities truthful and discoverable, reduces unnecessary duplicated
bookkeeping, and makes effects, devices, controls, and themes easier to add.
It preserves hardware-specific behavior where duplication is intentional and
advances through small, reviewed milestones rather than a giant rewrite.

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
  the ownership reason beside each affected control.
- Define semantic theme token values in themes and style components once.
- Give shared effect metadata one canonical source.
- Keep software-rendered effects distinct from similarly named native firmware
  effects.
- Keep hardware protocols and device-specific behavior owned by device
  packages.
- Remove duplicated bookkeeping where data can safely be derived.
- Preserve intentional backend-specific implementations.
- Prefer narrow migrations with focused tests and review.
- Retain legacy routes as fallbacks until replacement work is complete.

## 2. Completed milestones

### Devices workspace

- [x] `70872dc6` — Add isolated Devices workspace
- [x] `00ab0666` — Add device selection workspace
- [x] `1897a739` — Add OpenRGB workspace overview

These commits established the modern three-region Devices workspace, added
server-rendered device selection, and introduced an overview for imported
controllers. Existing legacy device pages remain available; not every legacy
device control has migrated.

### Mutation and persistence hardening

- [x] `e44a2bb3` — Harden OpenRGB lighting mutations
- [x] `e1770a8c` — Fix OpenRGB static override output
- [x] `e116554d` — Preserve OpenRGB temperature middle color
- [x] `1fad3c99` — Harden OpenRGB RGB override persistence
- [x] `426f262b` — Copy OpenRGB override into user profiles

These changes hardened LumenForge lighting mutations, override output, and
profile persistence for OpenRGB-imported devices. OpenRGB is the hardware
communication backend; it is not the source of LumenForge software animations.

### Capability and read-only workspace

- [x] `bfc0c751` — Add OpenRGB lighting capability snapshot
- [x] `c02d1eae` — Add read-only OpenRGB lighting workspace

The capability snapshot distinguishes palette kinds, cluster ownership,
supported effects, current configuration, and effective output. The read-only
Lighting workspace presents that information without changing native-device
implementations.

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

### Effects unavailable to individual OpenRGB-imported devices

Fourteen existing LumenForge software effects are not currently selectable for
individual OpenRGB-imported devices:

- `arc`
- `comet`
- `datastream`
- `marquee`
- `nebula`
- `pastelspiralrainbow`
- `plasmacore`
- `rain`
- `rotarystack`
- `sequential`
- `spiralrainbow`
- `stardust`
- `tokyonight`
- `visor`

`spiralrainbow` and `pastelspiralrainbow` already have importer dispatch cases
but are absent from the current allowlist. The other twelve require explicit
importer dispatch cases before they become selectable. Blindly adding IDs is
incorrect because unknown importer dispatch currently falls back to Static.
Expansion must follow descriptor integration and renderer-contract coverage.

### Existing icons

- Every implemented stable effect ID has a matching SVG at
  `static/img/icons/rgb/{stable-id}.svg`.
- Existing SVGs hard-code `#5599FF`.
- External image loading prevents automatic `currentColor` inheritance.
- Reusing the icons is planned, but theme-aware recoloring requires a separate,
  deliberate implementation.

## 4. Active foundation work

- [x] Establish the canonical generic software-effect descriptor registry.
- [x] Migrate generic software-effect capability lookup to canonical
  descriptors (`287c0f89`).
- [x] Migrate generic software-effect persistent-speed support to canonical
  descriptors (`b895cb75`).
- [ ] Derive the compatible LumenForge software-effect catalogue for
  OpenRGB-imported devices from descriptor scope and confirmed dispatch
  support.
- [ ] Derive the RGB Cluster software-effect catalogue from descriptor scope.
- [ ] Remove redundant label fallback where descriptors provide labels.
- [ ] Use descriptor icon identity in presentation.
- [ ] Add consistency tests before deleting old lists.
- [ ] Keep renderer dispatch explicit until a safe dispatch architecture is
  designed.

The registry should replace duplicated metadata gradually, consumer by
consumer. Existing sources should not be deleted together in one broad commit.

## 5. Lighting workspace controls

Implement controls in this order:

1. [ ] Theme-aware brightness slider
2. [ ] Speed slider shown only when supported
3. [ ] Static single-color editor
4. [ ] Two-color Start/End editor
5. [ ] Temperature Start/Middle/End editor
6. [ ] Gradient editor
7. [ ] Generated-palette presentation
8. [ ] Local RGB override controls
9. [ ] Persistence and reload verification
10. [ ] Accessible pending, success, and failure states
11. [ ] Cluster-owned disablement beside every affected control

For OpenRGB-imported devices, RGB Override must be presented truthfully as
controller-wide unless a future capability explicitly proves finer-grained
control.

Control design expectations:

- Use custom, modern, theme-aware presentation while preserving native
  accessibility and keyboard behavior.
- Avoid stock-looking unstyled sliders and excessive animation or gimmicks.
- Consume semantic theme tokens.
- Use hardened request handling, a bounded timeout, restoration after failure,
  and duplicate-request prevention for every mutation.

## 6. Effect catalogue migration and expansion

- [ ] Use canonical descriptors for capability metadata.
- [x] Add missing explicit importer dispatch cases (`9fb3eab2`).
  Importer rendering now checks the current device's exact supported-effect
  catalogue immediately before dispatch. Unsupported preserved profile IDs
  remain stored unchanged but render the Static fallback; `spiralrainbow` and
  `pastelspiralrainbow` obey the same boundary while absent from the catalogue.
- [ ] Add focused renderer contract coverage where it is missing.
- [ ] Expose all compatible LumenForge software effects to individual
  OpenRGB-imported devices.
- [ ] Verify behavior with 1, 2, 4, and 8 LEDs.
- [ ] Visually calibrate effects that are safe but limited on short buffers.
- [ ] Keep device-specific and firmware-native effect catalogues separate.
- [ ] Migrate the RGB Cluster catalogue to scope-based filtering.
- [ ] Remove superseded duplicated lists only after parity tests pass.

An effect looking better in a cluster is not sufficient reason to classify it
as cluster-only.

## 7. Effect icon integration

- [ ] Show the selected effect's existing icon in the Lighting workspace.
- [ ] Map icons by stable effect ID, never by display label.
- [ ] Keep the native select text-only.
- [ ] Retain an accessible text label.
- [ ] Provide a generic fallback only for an unknown mapping.
- [ ] Do not guess mappings from similar effect names.
- [!] Choose a deliberate rendering strategy:
  - inline and sanitize SVGs;
  - convert fill and stroke to `currentColor`;
  - use CSS masking; or
  - preserve the original blue assets.
- [ ] Validate icon presentation in every built-in theme.

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
| Speed support | `HasSpeedControl`, capability metadata, controls | Software-effect descriptors | `[~]` Registry created | Persistence and UI parity tests pass |
| Sensor requirement | Renderer dispatch and device-specific paths | Generic descriptors where applicable | `[~]` Generic CPU/GPU metadata created | Generic consumers migrate; device-local sensors remain separate |
| Topology and scope | Renderer knowledge and target-specific lists | Software-effect descriptors | `[~]` Registry created | Catalogue filtering and parity tests pass |
| Device-target effect list | OpenRGB-imported-device allowlist and native lists | Scope-filtered generic descriptors plus native capabilities | `[ ]` Planned | Explicit dispatch and compatibility tests exist |
| RGB Cluster effect list | Cluster catalogue | Scope-filtered generic descriptors | `[ ]` Planned | Cluster list and behavior parity tests pass |
| Icon identity | Stable asset naming and presentation paths | Software-effect descriptors | `[~]` Identity recorded | Presentation uses descriptor identity with fallback tests |
| Renderer dispatch | Importer and cluster switches | Explicit dispatch until a proven replacement exists | `[!]` Decision required | Output parity and lifecycle behavior are demonstrated |
| Default profile values | `database/rgb.json` and device-local defaults | Shipped or device-owned profiles | Intentional separation | Change only for a concrete profile requirement |
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
6. [ ] Expand compatible LumenForge software effects to individual
   OpenRGB-imported devices.
7. [ ] Integrate selected-effect icons.
8. [ ] Add theme-aware brightness control.
9. [ ] Add conditional speed control.
10. [ ] Add palette and color editors.
11. [ ] Add local override controls.
12. [ ] Migrate the RGB Cluster catalogue to descriptors.
13. [ ] Remove superseded duplicated bookkeeping after parity tests.
14. [ ] Add collapsible global navigation.
15. [ ] Expand native-device workspace adapters.
16. [ ] Consolidate global Dashboard telemetry.
17. [ ] Continue responsive, accessibility, and polish review.

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
- Do not require every effect to look ideal at every LED count.
- Do not restore Docker as part of the UI work.
- Do not add a schema migration unless a future feature truly requires one.

## 16. Open decisions

- [!] Final theme-aware SVG strategy
- [!] Visual treatment for generated-palette effects
- [!] Exact custom slider interaction and appearance
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
