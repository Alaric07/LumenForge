package m65rgbultra

import (
	"bytes"
	"path/filepath"
	"sync"
	"testing"
)

// Enable before hardware readiness so no one-shot blank competes with the
// test's explicitly controlled external queue consumer.
func openRGBQueueFixture(t *testing.T) (*Device, uint64) {
	t.Helper()
	d, _ := lightingFixture(t, true)
	if d.ProcessSetOpenRgbIntegration(true) != 1 {
		t.Fatal("enable OpenRGB")
	}
	readyLighting(d)
	return d, d.externalGeneration
}

func TestOpenRGBQueueReplacesPendingFrameWithoutBlocking(t *testing.T) {
	d, generation := openRGBQueueFixture(t)
	if cap(d.externalQueue) != 1 {
		t.Fatal("expected one pending frame", cap(d.externalQueue))
	}
	oldest := []byte{1, 2, 3, 4, 5, 6}
	newest := []byte{11, 22, 33, 44, 55, 66}
	d.enqueueOpenRGB(oldest, generation)
	if len(d.externalQueue) != cap(d.externalQueue) {
		t.Fatal("queue not saturated")
	}
	done := make(chan struct{})
	go func() { d.enqueueOpenRGB(newest, generation); close(done) }()
	// No consumer exists to make space: completion proves saturated enqueue
	// neither waits for a receiver nor waits for HID.
	waitDone(t, done)
	newest[0] = 99
	frame := <-d.externalQueue
	if !bytes.Equal(frame.data, []byte{11, 22, 33, 44, 55, 66}) || frame.generation != generation || len(d.externalQueue) != 0 {
		t.Fatal("newest copied state did not replace oldest", frame)
	}
}

func TestOpenRGBBurstAppliesFinalClientState(t *testing.T) {
	d, generation := openRGBQueueFixture(t)
	firstApplied, release := make(chan struct{}), make(chan struct{})
	var releaseOnce sync.Once
	unblock := func() { releaseOnce.Do(func() { close(release) }) }
	defer unblock()
	frames := make(chan []byte, 3)
	d.lightingWrite = func(packet []byte) {
		if packet[1] == 1 {
			close(firstApplied)
			<-release
		}
		frames <- packet
	}
	d.enqueueOpenRGB([]byte{1, 2, 3, 4, 5, 6}, generation)
	d.startLightingQueue()
	waitDone(t, firstApplied)
	final := []byte{251, 252, 253, 254, 255, 250}
	enqueued := make(chan struct{})
	go func() {
		// The consumer is blocked in the first write for this entire burst.
		for i := 0; i < 1000; i++ {
			d.enqueueOpenRGB([]byte{2, byte(i), 3, 4, 5, 6}, generation)
		}
		d.enqueueOpenRGB(final, generation)
		close(enqueued)
	}()
	waitDone(t, enqueued)
	if len(d.externalQueue) != 1 {
		t.Fatal("pending backlog", len(d.externalQueue))
	}
	unblock()
	first := waitFrame(t, frames)
	last := waitFrame(t, frames)
	if first[1] != 1 || !bytes.Equal(last, []byte{254, 251, 255, 255, 252, 255, 250, 253, 255}) {
		t.Fatal("burst final state lost", first, last)
	}
	// No further producers remain and there was only one pending slot, so this
	// second write is the final applied state, without a timing-based absence check.
	if len(d.externalQueue) != 0 {
		t.Fatal("historical backlog retained")
	}
}

func TestOpenRGBQueuePreservesSessionInvalidation(t *testing.T) {
	for _, transition := range []string{"ownership", "profile"} {
		t.Run(transition, func(t *testing.T) {
			d, oldGeneration := openRGBQueueFixture(t)
			old := []byte{71, 72, 73, 74, 75, 76}
			d.enqueueOpenRGB(old, oldGeneration)
			// Retain one dequeued old frame, then queue another for invalidation to drain.
			dequeued := <-d.externalQueue
			d.enqueueOpenRGB(old, oldGeneration)
			if transition == "ownership" {
				if d.ProcessSetOpenRgbIntegration(false) != 1 || d.ProcessSetOpenRgbIntegration(true) != 1 {
					t.Fatal("owner transition")
				}
			} else {
				p := cloneLightingProfile(*d.DeviceProfile)
				p.Path = filepath.Join(filepath.Dir(p.Path), "next.json")
				p.Active = false
				d.UserProfiles["next"] = &p
				if d.ChangeDeviceProfile("next") != 1 {
					t.Fatal("profile transition")
				}
			}
			d.stopLighting() // retire transition blank; external session stays active
			// Join any already-validated one-shot write before installing the output hook.
			d.deviceLock.Lock()
			d.deviceLock.Unlock()
			generation := d.externalGeneration
			if generation == oldGeneration || len(d.externalQueue) != 0 {
				t.Fatal("session was not invalidated/drained")
			}
			final := []byte{11, 22, 33, 44, 55, 66}
			d.enqueueOpenRGB(final, generation)
			d.enqueueOpenRGB(old, oldGeneration) // stale producer must not evict the new state
			queued := <-d.externalQueue
			if queued.generation != generation || !bytes.Equal(queued.data, final) {
				t.Fatal("old callback displaced current state", queued)
			}
			d.enqueueOpenRGB(queued.data, queued.generation)
			frames := make(chan []byte, 4)
			d.lightingWrite = func(packet []byte) { frames <- packet }
			// Even a frame taken before the transition must fail the final session gate.
			d.applyLightingPacket(composeLightingPacket(dequeued.data), "openrgb", dequeued.generation, nil)
			if len(frames) != 0 {
				t.Fatal("dequeued previous-owner frame wrote")
			}
			d.startLightingQueue()
			packet := waitFrame(t, frames)
			if !bytes.Equal(packet, []byte{44, 11, 255, 55, 22, 255, 66, 33, 255}) {
				t.Fatal("wrong current-session state", packet)
			}
		})
	}
}
