package main

import (
	"log"
	"net/http"

	"h12.me/egress/local"
)

func main() {
	var opt option
	opt.parse()
	srv, err := local.NewEgress(opt.Remote, opt.Dir, opt.Fetch)
	if err != nil {
		log.Fatal(err)
	}
	log.Printf("Egress local server starts listening on http://0.0.0.0:%s.", opt.Port)
	http.ListenAndServe("0.0.0.0:"+opt.Port, srv)
}
