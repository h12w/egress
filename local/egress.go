package local

import (
	"log"
	"net/http"
	"path"
)

type Egress struct {
	httpClient http.Client
	gae        *gaeDelegate
}

func NewEgress(remote, dir string) (*Egress, error) {
	var e Egress
	certs, err := newCertPool(path.Join(dir, "gae"))
	if err != nil {
		return nil, err
	}
	e.gae = &gaeDelegate{
		client: &e.httpClient,
		remote: remote,
		certs:  certs}
	return &e, err
}

func (e *Egress) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	log.Printf("%s %s %s", r.Method, r.RequestURI, r.Proto)
	e.gae.serve(w, r)
}
