package local

import (
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"
	"time"

	"h12.me/errors"
)

type Egress struct {
	fetcher
	connector
}

func NewEgress(remote *url.URL, dir, fetch string) (*Egress, error) {
	var err error
	httpClient := &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 15 * time.Second,
		}}
	remote.Path = path.Join(remote.Path, "f")
	var fetcher fetcher
	switch fetch {
	case "direct":
		fetcher = &directFetcher{httpClient}
		log.Print("fetch directly only!")
	case "remote":
		fetcher = &remoteFetcher{httpClient, remote.String()}
		log.Print("fetch from remote only")
	default:
		fetcher, err = newSmartFetcher(httpClient, remote.String(), path.Join(dir, "blocklist"))
	}
	if err != nil {
		return nil, err
	}
	certs, err := newCertPool(path.Join(dir, "cert"))
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
	cli, err := hijack(w)
	if err != nil {
		return err
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
		return nil, errors.Wrap(err)
	}
	return conn, nil
}
