package nodewatcher

import (
	"testing"

	"github.com/matarc/filewatcher/shared"
)

func TestNewPathManager(t *testing.T) {
	pm := NewPathManager(make(chan []shared.Operation))
	if pm == nil {
		t.Fatal("pm should not be nil")
	}
	if pm.operations == nil {
		t.Fatal("events channel shouldn't be nil")
	}
	if pm.quitCh == nil {
		t.Fatal("quitCh channel shouldn't be nil")
	}
}

func Test_handleList(t *testing.T) {
	pathCh := make(chan []shared.Operation)
	pm := NewPathManager(pathCh)

	eventsCh := pm.GetChan()
	go func() {
		eventsCh <- []shared.Operation{shared.Operation{"/my/path", shared.Create}}
		eventsCh <- []shared.Operation{shared.Operation{"/your/path", shared.Remove}}
	}()
	path := <-pathCh
	if path[0].Path != "/my/path" {
		t.Fatalf("path.path should be '/my/path', instead is '%s'", path[0].Path)
	}
	if path[0].Event != shared.Create {
		t.Fatalf("path.event should be 'Create', instead is '%s'", path[0].Event)
	}
	path = <-pathCh
	if path[0].Path != "/your/path" {
		t.Fatalf("path.path should be '/your/path', instead is '%s'", path[0].Path)
	}
	if path[0].Event != shared.Remove {
		t.Fatalf("path.event should be 'Remove', instead is '%s'", path[0].Event)
	}
}

func TestGetEventsChan(t *testing.T) {
	pm := NewPathManager(make(chan []shared.Operation))
	if pm.GetChan() == nil {
		t.Fatal("GetEventsChan should not return a nil channel")
	}
}
