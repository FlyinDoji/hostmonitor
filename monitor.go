package hostmonitor

import (
	"context"
	"net"
	"net/http"
	"net/url"
	"time"
)

const (
	monitorWriteQueueSize = 50   //Monitor map write operations queue
	stateUpdateQueueSize  = 1000 //State map update queue size
)

// monitorSet  encapsulates all the active monitors submitted to the engine
// Monitor object methods are thread safe because the map stores pointers
// Write operations are synchronized by the Manager goroutine
type monitorSet struct {
	Entries map[uint]monitor
}

// AddPipe funnels add operation requests to the Manager goroutine
type AddPipe struct {
	ch chan addOp
}

// DelPipe funnels delete operation requests to the Manager goroutine
type DelPipe struct {
	ch chan delOp
}

/* Operations encapsulate the operand and channel through which the operation result is retreived */
type addOp struct {
	m  monitor
	st stateAddOp
	r  chan bool
}
type delOp struct {
	id uint
	st stateDelOp
	r  chan bool
}
type stateAddOp struct {
	s monitorState
	r chan bool
}
type stateDelOp struct {
	id uint
	r  chan bool
}

/* Monitors share utility methods and implement custom testing behaviour */
type monitor interface {
	Stopper() chan int
	ID() uint
	Timeout() time.Duration
	setTicker() *time.Ticker

	// Multiple implementations
	execute(context.Context) (*http.Response, error)
	errHandler(error, time.Time) monitorState
	respHandler(*http.Response, int64, time.Time) monitorState
}

type baseMonitor struct {
	id        uint
	frequency time.Duration
	stopper   chan int
}

type httpMonitor struct {
	*baseMonitor
	url     string
	timeout time.Duration
}

type httpPostMonitor struct {
	*httpMonitor
	payload map[string]string
}

/* Constructors */
func newbaseMonitor(id uint, frequency time.Duration) *baseMonitor {
	return &baseMonitor{id, frequency, make(chan int)}
}

func newHTTPMonitor(id uint, frequency time.Duration, url string, timeout time.Duration) *httpMonitor {
	return &httpMonitor{newbaseMonitor(id, frequency), url, timeout}
}

func newHTTPPostMonitor(ht *httpMonitor, payload map[string]string) *httpPostMonitor {
	return &httpPostMonitor{ht, payload}
}

func newMonitorSet() *monitorSet {
	return &monitorSet{Entries: make(map[uint]monitor)}
}

/* Monitor interface methods */
/* Helpers */
func (bm *baseMonitor) ID() uint {
	return bm.id
}

func (bm *baseMonitor) Stopper() chan int {
	return bm.stopper
}

func (bm *baseMonitor) setTicker() *time.Ticker {
	return time.NewTicker(bm.frequency)
}

func (hm *httpMonitor) Timeout() time.Duration {
	return hm.timeout
}

/* Execute implementations */
func (hm *httpMonitor) execute(ctx context.Context) (*http.Response, error) {

	req, err := http.NewRequestWithContext(ctx, "GET", hm.url, nil)
	if err != nil {
		return nil, err
	}
	return http.DefaultClient.Do(req)
}

/* Result handlers */
func (hm *httpMonitor) respHandler(r *http.Response, respTime int64, timestamp time.Time) monitorState {
	r.Body.Close()
	return newHTTPResultState(hm.ID(), true, "", respTime, timestamp, r.StatusCode, r.Status)
}
func (bm *baseMonitor) errHandler(e error, timestamp time.Time) monitorState {

	msg := "unknown error"

	switch e := e.(type) {

	case *url.Error:
		if e.Timeout() {
			msg = "timeout"
			break
		}

		switch uw := e.Unwrap().(type) {

		case *net.OpError:
			if uw.Op == "dial" {
				msg = "no such host"
			} else if uw.Op == "read" {
				msg = "connection refused"
			} else {
				msg = "OpError"
			}

		case *net.AddrError:
			msg = "AddrError"

		case *net.DNSError:
			msg = "DNSError"
		}

	}

	return newHTTPResultState(bm.ID(), false, msg, 0, timestamp, 0, "")
}

// Context, execution, timing and handler calling
func launchTest(m monitor, resQ chan monitorState) {

	ctx, cancel := context.WithTimeout(context.Background(), m.Timeout())
	defer cancel()

	start := time.Now()
	r, err := m.execute(ctx)
	latency := time.Since(start).Milliseconds()
	timestamp := time.Now()

	if err != nil {
		resQ <- m.errHandler(err, timestamp)
		return
	}
	resQ <- m.respHandler(r, latency, timestamp)
}

/* Set helper functions */
func (mSet *monitorSet) add(m monitor) bool {

	if _, ok := mSet.Entries[m.ID()]; !ok {
		mSet.Entries[m.ID()] = m
		return true
	}
	return false
}

// Avoid waiting for slow tests to finish by sending the stop signal through a goroutine
func (mSet *monitorSet) delete(id uint) bool {

	if m, ok := mSet.Entries[id]; ok {
		go func() {
			close(m.Stopper())
			delete(mSet.Entries, id)
		}()
		return true
	}
	return false
}

/* Monitor goroutine scheduler */
func schedule(m monitor, resQ chan monitorState) {

	monitorTicker := m.setTicker()
	stop := m.Stopper()
	go func() {

		//fmt.Println("scheduled", m.ID())
		launchTest(m, resQ)

		for {
			select {
			case <-monitorTicker.C:
				launchTest(m, resQ)

			case <-stop:
				//fmt.Println("unscheduled", m.ID())
				monitorTicker.Stop()
				return
			}
		}

	}()

}

//Manager synchronously handles all the thread-unsafe operations on the cmd Map
func Manager(exit chan bool) (AddPipe, DelPipe, ReadPipe) {

	monitorSet := newMonitorSet()
	add := AddPipe{ch: make(chan addOp, monitorWriteQueueSize)}
	del := DelPipe{ch: make(chan delOp, monitorWriteQueueSize)}

	smExit := make(chan bool)
	stateAdd, stateDel, stateUpdate, read := stateManager(smExit)

	go func() {

		for {
			select {

			case data := <-add.ch:
				ok := monitorSet.add(data.m)
				if ok {
					stateAdd <- data.st
					<-data.st.r //wait for state to be added before scheduling
					schedule(data.m, stateUpdate)
				}
				data.r <- ok

			case data := <-del.ch:
				ok := monitorSet.delete(data.id)
				if ok {
					stateDel <- data.st
					<-data.st.r // wait for state to be cleared before processing new request
				}
				data.r <- ok

			case <-exit:
				close(smExit)
				return
			}
		}
	}()

	return add, del, read
}
