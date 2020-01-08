package main

import (
	"flag"
	"fmt"
	"net/http"

	"engine/monitor"
)

func main() {

	managerStop := make(chan bool)

	add, del, read := monitor.Manager(managerStop)
	r := monitor.NewRouter(add, del, read)

	port := flag.String("port", "8085", "API port")
	flag.Parse()

	addr := "127.0.0.1:" + *port
	fmt.Println("Listening on ", addr)
	err := http.ListenAndServe(addr, r)

	if err != nil {
		fmt.Println(err.Error())
		managerStop <- true
	}

}
