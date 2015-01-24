package local

import (
	"bufio"
	"bytes"
	"errors"
	"io"
	"log"
	"net"
	"net/http"

	"h12.me/egress/protocol"
)

type gaeDelegate struct {
	client *http.Client
	remote string
	certs  *certPool
}

func (g *gaeDelegate) serve(w http.ResponseWriter, r *http.Request) {
	if r.Method == "CONNECT" {
		g.serveHTTPS(w, r)
	} else {
		g.serveHTTP(w, r)
	}
}

func (g *gaeDelegate) serveHTTP(w http.ResponseWriter, r *http.Request) {
	resp, err := g.fetch(r)
	if err != nil {
		log.Printf("fail to fetch: %s", err.Error())
		return
	}
	defer resp.Body.Close()

	copyHeader(w.Header(), resp.Header)
	w.WriteHeader(resp.StatusCode)
	_, err = io.Copy(w, resp.Body)
	if err != nil {
		log.Printf("fail to copy response body: %s", err.Error())
		return
	}
}
func copyHeader(dst, src http.Header) {
	for k, v := range src {
		dst[k] = v
	}
}

func (g *gaeDelegate) serveHTTPS(w http.ResponseWriter, r *http.Request) {
	conn, err := hijack(w)
	if err != nil {
		log.Printf("fail to hijack: %s", err.Error())
		return
	}
	defer conn.Close()

	// assume the client want to connect in HTTPS
	host, _, err := net.SplitHostPort(r.URL.Host)
	if err != nil {
		log.Printf("SplitHostPort: %s", err.Error())
		return
	}
	sconn, err := fakeHTTPSHandeshake(conn, host, g.certs)
	if err != nil {
		log.Printf("fail to handleshake with TLS: %s", err.Error())
		return
	}
	defer sconn.Close()

	creq, err := http.ReadRequest(bufio.NewReader(sconn))
	if err != nil {
		if err != io.EOF {
			log.Printf("ReadRequest: %s", err.Error())
		}
		return
	}

	creq.URL.Scheme = "https" // set scheme back to https
	creq.URL.Host = creq.Host // fill URL.Host as a proxy request
	cresp, err := g.fetch(creq)
	if err != nil {
		log.Printf("fail to fetch: %s", err.Error())
		return
	}
	defer cresp.Body.Close()

	err = cresp.Write(sconn)
	if err != nil {
		if err != io.EOF {
			log.Printf("Write: %s", err.Error())
		}
		return
	}
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

func (g *gaeDelegate) fetch(req *http.Request) (*http.Response, error) {
	var buf bytes.Buffer
	if err := protocol.MarshalRequest(req, &buf); err != nil {
		return nil, err
	}
	resp, err := g.client.Post(g.remote, "application/data", &buf)
	if err != nil {
		return nil, err
	}
	// UnmarshalResponse will do resp.Body.Close
	return protocol.UnmarshalResponse(resp.Body, req)
}
