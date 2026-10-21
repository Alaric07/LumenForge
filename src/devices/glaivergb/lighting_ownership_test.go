package glaivergb

import (
	"LumenForge/src/rgb"
	"go/ast"
	"go/parser"
	"go/token"
	"path/filepath"
	"testing"
	"time"
)

func awaitLightingAction(t *testing.T, done <-chan struct{}) {
	t.Helper()
	select {
	case <-done:
	case <-time.After(2 * time.Second):
		t.Fatal("Lighting operation blocked on legacy Exit")
	}
}

func TestCanonicalAnimatedOwnershipIsSeparateFromLegacy(t *testing.T) {
	for _, preserveLegacy := range []bool{false, true} {
		name := "nil"
		if preserveLegacy {
			name = "legacy-owner-without-receiver"
		}
		t.Run(name, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			var prior *rgb.ActiveRGB
			if preserveLegacy {
				prior = rgb.Exit()
			}
			d.activeRgb = prior
			requireLighting(t, d.SetLightingEffect("rainbow"))
			frames := make(chan []byte, 1)
			d.lightingWrite = func(p []byte) {
				select {
				case frames <- p:
				default:
				}
			}
			d.Connected = true
			d.setDeviceColor()
			waitFrame(t, frames)
			d.lightingMu.Lock()
			worker := d.lightingWorker
			legacy := d.activeRgb
			d.lightingMu.Unlock()
			if worker == nil || legacy != prior {
				t.Fatal("canonical animation published into legacy ownership")
			}
			done := make(chan struct{})
			go func() { d.stopLighting(); close(done) }()
			awaitLightingAction(t, done)
			awaitLightingAction(t, worker.done)
			d.lightingMu.Lock()
			legacy = d.activeRgb
			current := d.lightingWorker
			d.lightingMu.Unlock()
			if legacy != prior || current != nil {
				t.Fatal("canonical cancellation/retirement touched legacy state")
			}
		})
	}
}

func TestCanonicalAnimationTransitionsIgnoreLegacyExit(t *testing.T) {
	for _, transition := range []string{"profile", "cluster", "openrgb"} {
		t.Run(transition, func(t *testing.T) {
			d, _ := lightingFixture(t, true)
			// Deliberately no receiver: a mistaken legacy send must hit the bounded test.
			legacy := rgb.Exit()
			d.activeRgb = legacy
			requireLighting(t, d.SetLightingEffect("rainbow"))
			frames := make(chan []byte, 1)
			d.lightingWrite = func(p []byte) {
				select {
				case frames <- p:
				default:
				}
			}
			p := cloneLightingProfile(*d.DeviceProfile)
			p.Path = filepath.Join(t.TempDir(), "next.json")
			d.UserProfiles["next"] = &p
			d.Connected = true
			d.setDeviceColor()
			waitFrame(t, frames)
			d.lightingMu.Lock()
			old := d.lightingWorker
			d.lightingMu.Unlock()
			result := make(chan uint8, 1)
			done := make(chan struct{})
			go func() {
				defer close(done)
				switch transition {
				case "profile":
					result <- d.ChangeDeviceProfile("next")
				case "cluster":
					result <- d.ProcessSetRgbCluster(true)
				case "openrgb":
					result <- d.ProcessSetOpenRgbIntegration(true)
				}
			}()
			awaitLightingAction(t, done)
			if <-result != 1 {
				t.Fatal("transition failed")
			}
			awaitLightingAction(t, old.done)
			d.lightingMu.Lock()
			unchanged := d.activeRgb == legacy
			d.lightingMu.Unlock()
			if !unchanged {
				t.Fatal("transition touched legacy owner")
			}
		})
	}
}

func TestCanonicalStaleRetirementOnlyTouchesCanonicalOwner(t *testing.T) {
	d, _ := lightingFixture(t, true)
	legacy := rgb.Exit()
	d.activeRgb = legacy
	old := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
	newer := &lightingWorker{stop: make(chan struct{}), done: make(chan struct{})}
	d.lightingWorker = newer
	d.retireLightingWorker(old)
	if d.lightingWorker != newer || d.activeRgb != legacy {
		t.Fatal("stale retirement changed ownership")
	}
	d.retireLightingWorker(newer)
	if d.lightingWorker != nil || d.activeRgb != legacy {
		t.Fatal("current retirement touched legacy")
	}
}

func TestLegacyFallbackRetainsAssignmentReceiverAndStop(t *testing.T) {
	// The untouched legacy renderer has no inert output hook. Pin its assignment
	// and receive structurally, then exercise its real fallback StopDirty path.
	file, err := parser.ParseFile(token.NewFileSet(), "glaivergb.go", nil, 0)
	requireLighting(t, err)
	assignment, receiver := false, false
	ast.Inspect(file, func(n ast.Node) bool {
		fn, ok := n.(*ast.FuncDecl)
		if !ok {
			return true
		}
		if fn.Name.Name != "setLegacyDeviceColor" {
			return false
		}
		ast.Inspect(fn.Body, func(node ast.Node) bool {
			if assign, ok := node.(*ast.AssignStmt); ok && len(assign.Lhs) == 1 && len(assign.Rhs) == 1 {
				lhs, ok := assign.Lhs[0].(*ast.SelectorExpr)
				call, callOK := assign.Rhs[0].(*ast.CallExpr)
				if ok && callOK && lhs.Sel.Name == "activeRgb" {
					if selector, ok := call.Fun.(*ast.SelectorExpr); ok && selector.Sel.Name == "Exit" {
						if pkg, ok := selector.X.(*ast.Ident); ok && pkg.Name == "rgb" {
							assignment = true
						}
					}
				}
			}
			if expr, ok := node.(*ast.UnaryExpr); ok && expr.Op == token.ARROW {
				if selector, ok := expr.X.(*ast.SelectorExpr); ok && selector.Sel.Name == "Exit" {
					if owner, ok := selector.X.(*ast.SelectorExpr); ok && owner.Sel.Name == "activeRgb" {
						receiver = true
					}
				}
			}
			return true
		})
		return false
	})
	if !assignment || !receiver {
		t.Fatal("legacy renderer assignment/Exit receive changed")
	}
	d, _ := lightingFixture(t, false)
	if d.attachLightingSource(nil) == nil || d.lightingAttached() {
		t.Fatal("partial attachment")
	}
	legacy := rgb.Exit()
	if cap(legacy.Exit) != 0 {
		t.Fatal("legacy Exit channel became buffered")
	}
	d.activeRgb = legacy
	d.timerKeepAlive = &time.Ticker{}
	d.timer = &time.Ticker{}
	received, done := make(chan struct{}), make(chan struct{})
	go func() {
		if !<-legacy.Exit {
			panic("unexpected legacy stop value")
		}
		close(received)
	}()
	go func() { d.StopDirty(); close(done) }()
	awaitLightingAction(t, done)
	awaitLightingAction(t, received)
	if d.activeRgb != legacy {
		t.Fatal("legacy StopDirty ownership behavior changed")
	}
	if _, open := <-legacy.Exit; open {
		t.Fatal("legacy Stop did not close Exit after send")
	}
}

func TestCanonicalSettingsErrorNamesGlaive(t *testing.T) {
	d, _ := lightingFixture(t, true)
	_, err := d.resolveLightingSettings("mouse")
	if err == nil || err.Error() != `unsupported GLAIVE RGB settings "mouse"` {
		t.Fatal("incorrect product error", err)
	}
}
