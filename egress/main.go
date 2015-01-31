package main

import (
	"log"
	"net/http"
	"time"

	"h12.me/egress"
	"h12.me/egress/local"
	"h12.me/egress/protocol"
)

func init() {
	protocol.NewWriter = egress.NewWriter
	protocol.NewReader = egress.NewReader
}

func main() {
	var opt option
	opt.parse()
	egress, err := local.NewEgress(opt.Remote, opt.Dir, opt.Fetch)
	if err != nil {
		log.Fatal(err)
	}
	srv := http.Server{
		Addr:         "0.0.0.0:" + opt.Port,
		Handler:      egress,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Printf("Egress local server starts listening on http://0.0.0.0:%s.", opt.Port)
	srv.ListenAndServe()
}
