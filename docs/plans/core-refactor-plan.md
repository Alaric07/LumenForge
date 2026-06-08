Summary & scope
- Repo inspected: RGB effects, OpenRGB imports, device/profile persistence, cluster, systray, temperatures, and UI handlers.
- Goal: identify duplicated/repeated code and propose a safe, incremental refactor plan that preserves runtime behavior.

Low-risk cleanup
1) Effect timing normalization
- Files: src/rgb/* (e.g., rainbow.go, wave.go)
- Duplication: elapsed/time and speedFactor calculations repeated.
- Helper: rgb.EffectClock { ElapsedSeconds(start), SpeedFactor(r, default) }
- Risk: low. Tests: unit tests for fixed start times and speeds.
- Quick win: extract computeElapsed/SpeedFactor and replace in a few effects.

2) JSON decode + invalid body handling
- Files: many server handlers (src/server/server.go)
- Duplication: json.NewDecoder(r.Body).Decode(&req) + identical Response on error.
- Helper: server.decodeJsonOrRespond[T](w,r,out *T) bool
- Risk: low. Tests: httptest invalid JSON -> identical response.
- Quick win: refactor 2–3 handlers.
- Status: proof-of-concept completed.
- Note: decodeRequestBody was added in src/server/server.go. setOpenRGBImportBrightness, setOpenRGBImportEffect, and setOpenRGBImportSpeed were refactored. Manual UI testing passed for brightness, effect, and speed controls on an OpenRGB imported device. go test ./src/server returned [no test files].

3) Template execute error handling
- Files: server UI handlers
- Duplication: ExecuteTemplate + same error logging/Response.Send.
- Helper: server.executeTemplateOrRespond(w,tpl,data) bool
- Risk: low. Tests: simulate template error path.

4) Parsing helpers
- Files: temperatures, display, rgb modules
- Duplication: strconv.Atoi/ParseFloat + trim/err handling.
- Helper: common.AtoiTrim, common.ParseFloatTrim
- Risk: low. Tests: unit tests for edge cases.

5) Systray menu utilities
- Files: src/systray/tray.go
- Duplication: addMenuItem/addSubMenu/insert logic.
- Helper: systray.MenuBuilder with AddItem/AddSubMenu/InsertAfter
- Risk: low. Tests: unit tests assert menu state.

Medium-risk refactors
1) OpenRGB import handlers
- Files: src/server/server.go (setOpenRGBImport* endpoints)
- Duplication: JSON decode -> default serial -> device lookup -> identical responses.
- Helper: server.handleOpenRGBImport[T](w,r,defaultSerial string, action func(dev, payload) error)
- Risk: medium — centralizes request lifecycle; test with mocked device and httptest.
- Quick win: extract getDeviceFromJsonRequest and respondInvalidBody.

2) Profile load/save path and file ops
- Files: src/led/led.go, src/cluster/cluster.go, many device drivers
- Duplication: path construction using ConfigPath + "/database/<category>/<id>.json", open+decode, SaveJsonData usage and logging.
- Helper: profiles.Manager { ProfilePath, LoadProfile[T], SaveProfile, EnsureDefault }
- Risk: medium — persistence semantics must be preserved; test via temp ConfigPath.
- Quick win: extract profilePath(category,id) and thin wrappers for device drivers.

3) Cluster effect dispatch switch
- Files: src/cluster/cluster.go::generateRgbEffect
- Duplication: switch/case calling r.<Effect> then buff = r.Output.
- Helper: dispatch map map[string]func(*rgb.ActiveRGB,*time.Time)[]byte or small helper callEffectAndReturnOutput
- Risk: medium — must preserve side-effects and special cases (temperature, gradient args).
- Quick win: extract repeated call pattern into helper.

High-risk / avoid for now (require strong tests)
1) Per-effect output assembly
- Files: src/rgb/*.go (many effect implementations)
- Duplication: per-pixel writes to r.Buffer vs temporary map, AIO/HasLCD special-case index masking, then r.Raw = buf and r.Output = SetColor/SetColorInverted(buf)
- Suggested abstraction: rgb.OutputBuilder with Write(index, r,g,b) and Finalize() that sets r.Raw and r.Output
- Risk: high — touches runtime-critical rendering path; byte-order/offset/masking regressions would be visible on devices.
- Tests: golden outputs for representative effects (rainbow, wave, static, gradient) for fixed start time and seed; unit tests for AIO masking and inverted output.
- Safe plan: Stage 1 add golden tests, Stage 2 extract finalizeOutput helper, Stage 3 migrate to OutputBuilder incrementally.

2) Profile migrations manager
- Files: profile provisioning/upgrade across cluster, temperatures, lcd, etc.
- Duplication: "if !FileExists then SaveJsonData(default)" and ad-hoc upgrade heuristics.
- Suggested abstraction: profiles.Migrator with EnsureDefault and versioned migrations.
- Risk: high — could overwrite user data if applied incorrectly.
- Tests: integration tests in tempdir, backups before migration.

Incremental implementation plan (small, reviewable PRs)
1) Low-risk helpers (PR #1)
- Add: server.decodeJsonOrRespond, server.executeTemplateOrRespond, common.AtoiTrim.
- Replace: 2–3 server handlers and 1 UI handler, with unit tests using httptest and small template errors.

2) Profile helpers (PR #2)
- Add: profiles.Path and thin wrappers LoadLedProfile/SaveLedProfile.
- Replace: device driver one-liners.
- Tests: temp ConfigPath read/write.

3) Cluster dispatch cleanup (PR #3)
- Add: callEffectAndReturnOutput helper, replace repeated case bodies.
- Tests: per-profile outputs compared to pre-refactor outputs for a small set.

4) Systray utilities (PR #4)
- Add: MenuBuilder (internal package) and migrate add/insert functions.
- Tests: unit tests for ordering.

5) Output finalize (PR #5 — cautious)
- Add: finalizeOutput(r, buf) and replace finalization lines in a few effects; run golden tests.
- If safe, proceed to full OutputBuilder migration (PR #6) with broad golden/integration tests.

6) OpenRGB handler consolidation (PR #7)
- Add: handleOpenRGBImport generic wrapper and refactor endpoints incrementally with mocked device tests.

7) Migrations/EnsureDefault (PR #8)
- Create profiles.EnsureDefault and migrate modules conservatively with backups and tests.

Testing strategy
- Unit tests for all new helpers using httptest, temp directories, and deterministic inputs.
- Golden tests for RGB outputs: record outputs from key effects, run them after refactor and assert equal bytes.
- Integration smoke: run app in dev with simulated controllers where possible; verify systray/menu and UI pages render.

Next steps
- Continue PR #1 / Low-risk helpers.
- Do not modify this plan automatically.
- Next candidate: inspect whether setOpenRGBImportColor can safely use decodeRequestBody.
- Leave setOpenRGBImportConfig for later because it touches saved zone config/state.
- Do not start implementation without explicit approval.
