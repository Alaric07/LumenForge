package k65rgbRF

import (
	"LumenForge/src/keyboards"
	"encoding/json"
	"os"
	"reflect"
	"testing"
)

func TestKeyboardPresentationIgnoresPacketPositions(t *testing.T) {
	for _, suffix := range []string{"", "-de"} {
		t.Run(suffix, func(t *testing.T) {
			data, err := os.ReadFile("../../../database/keyboard/k65rgbRF" + suffix + ".json")
			if err != nil {
				t.Fatal(err)
			}
			var keyboard keyboards.Keyboard
			if err := json.Unmarshal(data, &keyboard); err != nil {
				t.Fatal(err)
			}
			d := &Device{DeviceProfile: &DeviceProfile{Profile: "fixture", Keyboards: map[string]*keyboards.Keyboard{"fixture": &keyboard}}}
			before := d.keyboardZoneEditor()
			// Exercise only the presentation helper: production validation still
			// requires the exact shipped packet map and is not weakened here.
			for id, row := range keyboard.Row {
				for keyID, key := range row.Keys {
					for i := range key.PacketIndex {
						key.PacketIndex[i] = (key.PacketIndex[i] + 71) % 168
					}
					row.Keys[keyID] = key
				}
				keyboard.Row[id] = row
			}
			if !reflect.DeepEqual(d.keyboardZoneEditor(), before) {
				t.Fatal("packet positions changed geometry")
			}
		})
	}
}
