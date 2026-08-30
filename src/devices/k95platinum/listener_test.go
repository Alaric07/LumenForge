package k95platinum

import "testing"

func TestK95ListenerDataFailsClosedWithoutHandle(t *testing.T) {
	if data := (&Device{}).getListenerData(nil); data != nil {
		t.Fatalf("listener data = %#v, want nil", data)
	}
}

func TestK95StopBackendListenerHandlesMissingListener(t *testing.T) {
	stop, done := make(chan struct{}), make(chan struct{})
	close(done)
	d := &Device{listenerStop: stop, listenerDone: done}
	d.stopBackendListener()
	select {
	case <-stop:
	default:
		t.Fatal("listener stop channel was not closed")
	}
}
