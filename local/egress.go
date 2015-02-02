package local

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"h12.me/egress/protocol"
	"h12.me/errors"
)

type Egress struct {
	fetcher
	connector
}

func NewEgress(remote *url.URL, workDir, fetcherType, connectorType string) (*Egress, error) {
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 15 * time.Second,
		}}
	blockList, err := newBlockList(path.Join(workDir, "blocklist"))
	if err != nil {
		return nil, err
	}
	fetcher, err := newFetcher(fetcherType, remote, httpClient, blockList)
	if err != nil {
		return nil, err
	}
	connector, err := newConnector(connectorType, remote, fetcher, blockList, workDir)
	if err != nil {
		return nil, err
	}

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
		return errors.Wrap(err)
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		if !isEOF(err) {
			return errors.Wrap(err)
		}
		return nil
	}
	return nil
}
func copyHeader(dst, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}

func (e *Egress) serveConnect(w http.ResponseWriter, req *http.Request) error {
	cli, err := protocol.Hijack(w)
	if err != nil {
		return err
	}
	defer cli.Close()
	return e.connect(req, cli)
}
