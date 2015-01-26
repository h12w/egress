package local

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"path"
	"time"
)

type delegate interface {
	fetch(req *http.Request) (*http.Response, error)
	tunnel(host string, cli net.Conn) error
}

type Egress struct {
	httpClient http.Client
	gae        delegate
	direct     delegate
}

func NewEgress(remote, dir string) (*Egress, error) {
	var e Egress
	e.httpClient.Transport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout:   30 * time.Second,
			KeepAlive: 30 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 20 * time.Second,
	}

	e.direct = &directDelegate{&e.httpClient}
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

func (e *Egress) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	//log.Printf("%s directly %s %s", req.Method, req.RequestURI, req.Proto)
	//if err := e.serve(w, req, e.direct); err != nil {
	//	log.Printf("fail to fetch directly: %s", err.Error())
	//	log.Printf("%s via GAE %s %s", req.Method, req.RequestURI, req.Proto)
	if err := e.serve(w, req, e.gae); err != nil {
		log.Printf("fail to fetch via GAE: %s", err.Error())
	}
	//}
}

func (e *Egress) serve(w http.ResponseWriter, req *http.Request, de delegate) error {
	if req.Method == "CONNECT" {
		return e.tunnel(w, req, de)
	}
	return e.fetch(w, req, de)
}

func (e *Egress) fetch(w http.ResponseWriter, req *http.Request, de delegate) error {
	resp, err := de.fetch(req)
	if err != nil {
		return fmt.Errorf("fail to fetch: %s", err.Error())
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		return fmt.Errorf("fail to copy response body: %s", err.Error())
	}
	return nil
}
func copyHeader(dst, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}

func (e *Egress) tunnel(w http.ResponseWriter, req *http.Request, de delegate) error {
	cli, err := hijack(w)
	if err != nil {
		return fmt.Errorf("fail to hijack: %s", err.Error())
	}
	defer cli.Close()
	return de.tunnel(req.URL.Host, cli)
}
func hijack(w http.ResponseWriter) (net.Conn, error) {
	hij, ok := w.(http.Hijacker)
	if !ok {
		return nil, errors.New("cannot hijack the ResponseWriter")
	}
	conn, _, err := hij.Hijack()
	if err != nil {
		return nil, err
	}
	return conn, nil
}
