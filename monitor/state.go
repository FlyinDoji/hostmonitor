package monitor

import "time"

type stateSet struct {
	Entries map[uint]monitorState `json:"monitors"`
}

//ReadPipe exposes a pointer to the stateSet for marshalling into a JSON response
type ReadPipe struct {
	ch chan *stateSet
	r  chan bool
}

//monitorState encapsulates the results obtained from test execution
type monitorState interface {
	id() uint
	alive() bool
	message() string
	latency() int64
	timestamp() time.Time
	statusCode() int
	status() string

	update(monitorState)
}

type baseState struct {
	ID         uint      `json:"id"`
	Alive      bool      `json:"alive"`
	Message    string    `json:"message"`
	Change     string    `json:"change"`
	Latency    int64     `json:"latency"`
	LastChange time.Time `json:"lastChange"` // time when the last change of 'alive' occurred
	Timestamp  time.Time `json:"timestamp"`  // time at which last test was performed
}

type httpState struct {
	*baseState
	Code   int    `json:"code"`
	Status string `json:"status"`
}

/* Default constructors */
func newBaseState(id uint) *baseState {
	return &baseState{ID: id, Alive: true, Message: "new", Change: "", Latency: 0, LastChange: time.Now(), Timestamp: time.Now()}
}
func newHTTPState(id uint) *httpState {
	return &httpState{baseState: newBaseState(id), Code: 0, Status: ""}
}

func newStateSet() *stateSet {
	return &stateSet{Entries: make(map[uint]monitorState)}
}

/* Parameter constructors */
func newBaseResultState(id uint, alive bool, message string, latency int64, timestamp time.Time) *baseState {
	return &baseState{ID: id, Message: message, Change: "", Alive: alive, Latency: latency, LastChange: time.Now(), Timestamp: timestamp}
}
func newHTTPResultState(id uint, alive bool, message string, latency int64, timestamp time.Time, code int, status string) *httpState {
	return &httpState{baseState: newBaseResultState(id, alive, message, latency, timestamp), Code: code, Status: status}
}

/* Helpers */
func (s *baseState) alive() bool {
	return s.Alive
}

func (s *baseState) id() uint {
	return s.ID
}
func (s *baseState) timestamp() time.Time {
	return s.Timestamp
}
func (s *baseState) latency() int64 {
	return s.Latency
}
func (s *baseState) message() string {
	return s.Message
}
func (hs *httpState) statusCode() int {
	return hs.Code
}
func (hs *httpState) status() string {
	return hs.Status
}

func (hs *httpState) update(newState monitorState) {

	hs.Change = ""
	hs.Message = newState.message()
	hs.Timestamp = newState.timestamp()

	switch newState.alive() {
	case true:

		if !hs.Alive {
			hs.LastChange = newState.timestamp()
			hs.Change = "up"
		}

		hs.Latency = newState.latency()
		hs.Code = newState.statusCode()
		hs.Status = newState.status()

	case false:
		if hs.Alive {
			hs.LastChange = newState.timestamp()
			hs.Change = "down"
			hs.Latency = -1
			hs.Code = -1
			hs.Status = ""
		}

	}
	hs.Alive = newState.alive()
}

func (st *stateSet) add(s monitorState) {
	st.Entries[s.id()] = s
}

func (st *stateSet) delete(id uint) {
	delete(st.Entries, id)
}

// Update requests must first check if the State is still in the Set
// as the monitor might be deleted while a test is being performed
func stateManager(exit chan bool) (chan stateAddOp, chan stateDelOp, chan monitorState, ReadPipe) {

	stSet := newStateSet()
	add := make(chan stateAddOp, monitorWriteQueueSize)
	del := make(chan stateDelOp, monitorWriteQueueSize)
	update := make(chan monitorState, stateUpdateQueueSize)
	read := ReadPipe{make(chan *stateSet), make(chan bool)}

	go func() {

		for {
			select {
			case data := <-add:
				stSet.add(data.s)
				data.r <- true

			case data := <-del:
				stSet.delete(data.id)

				data.r <- true

			// Check existence before updating
			case data := <-update:
				if s, ok := stSet.Entries[data.id()]; ok {
					s.update(data)
				}

			// Wait for the handler to finish writing the response
			case <-read.r:
				read.ch <- stSet
				<-read.r

			case <-exit:
				return
			}
		}
	}()

	return add, del, update, read
}
