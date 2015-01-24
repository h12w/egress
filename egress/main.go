package main

import (
	"log"
	"net/http"

	"h12.me/egress/local"
)

func main() {
	var opt option
	opt.parse()
	srv, err := local.NewEgress(opt.Remote, opt.Dir)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("proxy started on 0.0.0.0:%s.", opt.Port)
	log.Printf("remote address is %s.", opt.Remote)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+opt.Port, srv))
}
