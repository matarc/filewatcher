package nodewatcher

import "github.com/matarc/filewatcher/shared"

type PathManager struct {
	operations chan []shared.Operation
	list       []shared.Operation
	quitCh     chan struct{}
}

// NewPathManager returns a path manager that is responsible for getting all files operations
// from the watcher and sending them to the `Client` once he is ready to send them to the storage
// server.
func NewPathManager(pathCh chan<- []shared.Operation) *PathManager {
	pm := new(PathManager)
	pm.operations = make(chan []shared.Operation, 10)
	pm.quitCh = make(chan struct{})
	go pm.handleList(pathCh)
	return pm
}

// GetChan returns a send only channel on which the `PathManager` will receive all file's
// operations.
func (pm *PathManager) GetChan() chan<- []shared.Operation {
	return pm.operations
}

// handleList keeps the list of all operations that haven't been sent to the storage server yet.
func (pm *PathManager) handleList(pathCh chan<- []shared.Operation) {
	var buf []shared.Operation
	dataSentCh := make(chan struct{})
	for {
		if len(buf) == 0 && len(pm.list) > 0 {
			buf = pm.list
			pm.list = []shared.Operation{}
			go func() {
				pathCh <- buf
				buf = []shared.Operation{}
				dataSentCh <- struct{}{}
			}()
		}
		select {
		case operation := <-pm.operations:
			pm.list = append(pm.list, operation...)
		case <-dataSentCh:
		case <-pm.quitCh:
			return
		}
	}
}

// Stop stops the `PathManager`.
func (pm *PathManager) Stop() {
	if pm.quitCh != nil {
		close(pm.quitCh)
		pm.quitCh = nil
	}
}
