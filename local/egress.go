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

type Egress struct {
	fetcher
	connector
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
	//fetcher := &gaeFetcher{
	//	client: httpClient,
	//	remote: remote,
	//}
	fetcher, err := newSmartFetcher(httpClient, remote, path.Join(dir, "blocklist"))
	if err != nil {
		return nil, err
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
	if req.Method == "CONNECT" {
		if err := e.serveConnect(w, req); err != nil {
			log.Print(err)
		}
	} else {
		if err := e.serveOthers(w, req); err != nil {
			log.Print(err)
		}
	}
}

func (e *Egress) serveOthers(w http.ResponseWriter, req *http.Request) error {
	resp, err := e.fetch(req)
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

func (e *Egress) serveConnect(w http.ResponseWriter, req *http.Request) error {
	cli, err := hijack(w)
	if err != nil {
		return fmt.Errorf("fail to hijack: %s", err.Error())
	}
	defer cli.Close()
	return e.connect(req.URL.Host, cli)
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
