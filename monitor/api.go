package monitor

import (
	"fmt"
	"github.com/julienschmidt/httprouter"
	"net/http"
	"time"
)

type respMsg struct {
	Ok      bool
	Message string
}

func responseBodyWriter(w http.ResponseWriter, rb interface{}) {

	msg, err := marshalResponseBody(rb)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Error generating response body"))
		return
	}
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(http.StatusOK)
	w.Write(msg)
}

// AddHTTPMonitor adds a new monitor to the engine
func (add *AddPipe) addHTTPMonitor(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	rb := &respMsg{Ok: true, Message: "Monitor added"}
	defer responseBodyWriter(w, rb)

	b, err := unmarshalRequestBody(r.Body)
	if err != nil {
		rb.Ok = false
		rb.Message = err.Error()
		return
	}

	numArgs, strArgs, err := validArgs(b, []string{"id", "url", "frequency", "timeout"})
	if err != nil {
		rb.Ok = false
		rb.Message = err.Error()
		return
	}

	id := uint(numArgs["id"])
	freq := time.Duration(numArgs["frequency"]) * time.Second
	timeout := time.Duration(numArgs["timeout"]) * time.Second
	url := strArgs["url"]

	var m monitor
	var s monitorState
	switch p.ByName("method") {

	case "GET":
		m = newHTTPMonitor(id, freq, url, timeout)
		s = newHTTPState(id)

	case "POST":
		rb.Ok = false
		rb.Message = "POST not implemented"
		return

	}

	a := addOp{m: m, st: stateAddOp{s: s, r: make(chan bool)}, r: make(chan bool)}
	add.ch <- a

	if !<-a.r {
		rb.Ok = false
		rb.Message = fmt.Sprintf("Already exists id=%d", id)
	}

}

// deleteMonitor removes an existing monitor from the engine
func (del *DelPipe) deleteMonitor(w http.ResponseWriter, r *http.Request, p httprouter.Params) {

	rb := &respMsg{Ok: true, Message: "Monitor deleted"}
	defer responseBodyWriter(w, rb)

	b, err := unmarshalRequestBody(r.Body)
	if err != nil {
		rb.Ok = false
		rb.Message = err.Error()
		return
	}
	numArgs, _, err := validArgs(b, []string{"id"})
	if err != nil {
		rb.Ok = false
		rb.Message = err.Error()
		return
	}

	id := uint(numArgs["id"])

	d := delOp{id: id, st: stateDelOp{id: id, r: make(chan bool)}, r: make(chan bool)}
	del.ch <- d

	if !<-d.r {
		rb.Ok = false
		rb.Message = fmt.Sprintf("id=%d not registered", id)
	}

}

// StateReader retrieves the stateSet structure
func (rp *ReadPipe) readState(w http.ResponseWriter, _ *http.Request, _ httprouter.Params) {
	rp.r <- true
	rb := <-rp.ch
	responseBodyWriter(w, rb)
	rp.r <- true
}

// NewRouter initialises the http router
func NewRouter(a AddPipe, d DelPipe, r ReadPipe) *httprouter.Router {

	router := httprouter.New()
	router.POST("/addmonitor/http/:method", a.addHTTPMonitor)
	router.DELETE("/deletemonitor", d.deleteMonitor)
	router.GET("/monitors", r.readState)

	return router
}
