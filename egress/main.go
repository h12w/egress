package main

import (
	"log"
	"net/http"
	"net/url"
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
	remote, err := url.Parse(opt.Remote)
	if err != nil {
		log.Fatal(err)
	}
	egress, err := local.NewEgress(remote, opt.Dir, opt.Fetch, opt.Connect)
	if err != nil {
		log.Fatal(err)
	}
	srv := http.Server{
		Addr:         "0.0.0.0:" + opt.Port,
		Handler:      egress,
		ReadTimeout:  10 * time.Second,
		WriteTimeout: 10 * time.Second,
	}
	log.Print("Egress local server started.")
	log.Printf("        http://0.0.0.0:%s   ->   %s", opt.Port, opt.Remote)
	if err := srv.ListenAndServe(); err != nil {
		log.Print(err)
	}
}
