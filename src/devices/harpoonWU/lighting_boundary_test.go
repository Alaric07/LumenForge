package harpoonWU

import (
	"bytes"
	"crypto/sha256"
	"fmt"
	"go/ast"
	"go/parser"
	"go/token"
	"os"
	"strings"
	"testing"
)

func TestWUUSBTransportAndInitializationContract(t *testing.T) {
	data, err := os.ReadFile("harpoonWU.go")
	requireLighting(t, err)
	fset := token.NewFileSet()
	file, err := parser.ParseFile(fset, "harpoonWU.go", data, 0)
	requireLighting(t, err)
	methods := map[string]string{}
	for _, decl := range file.Decls {
		if fn, ok := decl.(*ast.FuncDecl); ok {
			methods[fn.Name.Name] = string(data[fset.Position(fn.Pos()).Offset:fset.Position(fn.End()).Offset])
		}
	}
	// Pin the independently audited direct-HID path: own mutex, 65-byte output,
	// report ID 0/byte1=08, command/header positions, Write + blocking Read. No
	// receiver delegation, timeout conversion or other transport edits are allowed.
	for name, want := range map[string]string{"transfer": "55e48d26a143fc5983a7fe0765b29c6e4de8ca255d36acee71c6964115f662bc", "getListenerData": "cbcd22f96dfbd11b8d53ea5d80be508544a29d150989b3483c7a9888919131a2"} {
		if got := fmt.Sprintf("%x", sha256.Sum256([]byte(methods[name]))); got != want {
			t.Fatalf("WU %s transport changed: %s", name, got)
		}
	}
	if bufferSize != 64 || bufferSizeWrite != 65 || headerSize != 2 || headerWriteSize != 4 || deviceKeepAlive != 20000 {
		t.Fatal("WU framing/cadence")
	}
	if !bytes.Equal(cmdSoftwareMode, []byte{1, 3, 0, 2}) || !bytes.Equal(cmdHardwareMode, []byte{1, 3, 0, 1}) || !bytes.Equal(cmdOpenEndpoint, []byte{13, 0, 1}) || !bytes.Equal(cmdWriteColor, []byte{6, 0}) {
		t.Fatal("WU commands")
	}
	init := methods["Init"]
	previous := -1
	for _, call := range []string{"hid.OpenPath(path)", "d.getManufacturer()", "d.getSerial()", "d.loadDeviceProfiles()", "d.saveDeviceProfile()", "d.attachLightingRuntime(", "d.getDeviceFirmware()", "d.setSoftwareMode()", "d.getBatterLevel()", "d.setAngleSnapping()", "d.setButtonOptimization()", "d.initLeds()", "d.setDeviceColor()", "d.toggleDPI()", "d.backendListener()", "d.setKeepAlive()", "d.loadKeyAssignments()", "d.setupKeyAssignment()", "d.setupOpenRGBController()", "d.setupClusterController()", "d.createDevice()", "d.startQueueWorker()"} {
		index := strings.Index(init, call)
		if index <= previous {
			t.Fatalf("WU startup order changed at %s", call)
		}
		previous = index
	}
	// The audited USB Init normalizes polling default 4 in profile bootstrap; it
	// never sent a polling command. Only UpdatePollingRate reconfigures hardware.
	if strings.Contains(init, "cmdSetPollingRate") || strings.Contains(init, "UpdatePollingRate(") {
		t.Fatal("invented startup polling command")
	}
	if strings.Contains(methods["ChangeDeviceProfile"], "cmdSetPollingRate") {
		t.Fatal("invented profile polling command")
	}
}

func TestWULogicalFramesReachUnchangedColorCommand(t *testing.T) {
	d, _ := lightingFixture(t, true)
	d.lightingWrite = nil
	packets := make(chan []byte, 20)
	d.lightingColorTransfer = func(command, data []byte, read bool) error {
		if !read || !bytes.Equal(command, []byte{6, 0}) {
			t.Error("color endpoint/read contract")
		}
		packets <- append([]byte(nil), data...)
		return nil
	}
	startTestUSB(d)
	// Clean canonical Static is cyan. Separate stage1 white indicator is raw.
	if got := waitFrame(t, packets); !bytes.Equal(got, []byte{6, 0, 0, 0, 255, 0, 255, 255, 255, 255}) {
		t.Fatal("Native USB frame", got)
	}
	for _, owner := range []string{"cluster", "openrgb"} {
		if owner == "cluster" {
			d.ProcessSetRgbCluster(true)
			d.writeColorCluster([]byte{11, 22, 33}, 0)
		} else {
			d.ProcessSetRgbCluster(false)
			waitFrame(t, packets)
			d.ProcessSetOpenRgbIntegration(true)
			waitFrame(t, packets)
			d.startLightingQueue()
			d.writeColorEx([]byte{11, 22, 33}, 0)
		}
		if got := waitFrame(t, packets); !bytes.Equal(got, []byte{6, 0, 0, 0, 255, 11, 255, 22, 255, 33}) {
			t.Fatal(owner, got)
		}
	}
}
