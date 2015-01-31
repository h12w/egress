package main

import (
	"log"
	"net/http"
	"time"

	"h12.me/egress/local"
	"h12.me/egress/protocol"
	"h12.me/egress/secret"
)

func init() {
	protocol.NewWriter = secret.NewWriter
	protocol.NewReader = secret.NewReader
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
