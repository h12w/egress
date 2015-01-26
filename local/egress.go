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

type fetcher interface {
	fetch(req *http.Request) (*http.Response, error)
}

type connector interface {
	connect(host string, cli net.Conn) error
}

type Egress struct {
	fetcher   fetcher
	connector connector
}

func NewEgress(remote, dir string) (*Egress, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   3 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 3 * time.Second,
		}}
	//fetcher := directFetcher{httpClient}
	fetcher := gaeFetcher{
		client: httpClient,
		remote: remote,
	}
	//fetcher := dualFetcher{
	//	directFetcher{httpClient},
	//	gaeFetcher{
	//		client: httpClient,
	//		remote: remote,
	//	},
	//}
	//connector := &directDelegate{fetcher}
	certs, err := newCertPool(path.Join(dir, "gae"))
	if err != nil {
		return nil, err
	}

	connector := &fakeTLSConnector{
		fetcher: fetcher,
		certs:   certs}
	return &Egress{fetcher, connector}, nil
}

func (e *Egress) ServeHTTP(w http.ResponseWriter, req *http.Request) {
	//log.Printf("%s directly %s %s", req.Method, req.RequestURI, req.Proto)
	//if err := e.serve(w, req, e.direct); err != nil {
	//	log.Printf("fail to fetch directly: %s", err.Error())
	//	log.Printf("%s via GAE %s %s", req.Method, req.RequestURI, req.Proto)
	if err := e.serve(w, req, e.connector); err != nil {
		log.Printf("fail to fetch: %s", err.Error())
	}
	//}
}

func (e *Egress) serve(w http.ResponseWriter, req *http.Request, de connector) error {
	if req.Method == "CONNECT" {
		return e.serveConnect(w, req, de)
	}
	return e.serveOthers(w, req, de)
}

func (e *Egress) serveOthers(w http.ResponseWriter, req *http.Request, de connector) error {
	resp, err := e.fetcher.fetch(req)
	if err != nil {
		w.WriteHeader(http.StatusGatewayTimeout)
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

func (e *Egress) serveConnect(w http.ResponseWriter, req *http.Request, de connector) error {
	cli, err := hijack(w)
	if err != nil {
		return fmt.Errorf("fail to hijack: %s", err.Error())
	}
	defer cli.Close()
	return de.connect(req.URL.Host, cli)
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
