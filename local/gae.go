package local

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"

	"h12.me/egress/protocol"
)

type gaeDelegate struct {
	client *http.Client
	remote string
	certs  *certPool
}

func (g *gaeDelegate) tunnel(hostPort string, cli net.Conn) error {
	// assume the client want to connect in HTTPS
	host, _, err := net.SplitHostPort(hostPort)
	if err != nil {
		return fmt.Errorf("SplitHostPort: %s", err.Error())
	}
	sconn, err := fakeHTTPSHandeshake(cli, host, g.certs)
	if err != nil {
		return fmt.Errorf("fail to handleshake with TLS: %s", err.Error())
	}
	defer sconn.Close()

	creq, err := http.ReadRequest(bufio.NewReader(sconn))
	if err != nil {
		if err != io.EOF {
			return fmt.Errorf("ReadRequest: %s", err.Error())
		}
		return nil
	}

	creq.URL.Scheme = "https" // set scheme back to https
	creq.URL.Host = creq.Host // fill URL.Host as a proxy request
	cresp, err := g.fetch(creq)
	if err != nil {
		return fmt.Errorf("fail to fetch: %s", err.Error())
	}
	defer cresp.Body.Close()

	err = cresp.Write(sconn)
	if err != nil {
		if err != io.EOF {
			return fmt.Errorf("Write: %s", err.Error())
		}
		return nil
	}
	return nil
}

func (g *gaeDelegate) fetch(req *http.Request) (*http.Response, error) {
	req, err := protocol.MarshalRequest(req, g.remote)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	// UnmarshalResponse will do resp.Body.Close
	return protocol.UnmarshalResponse(resp.Body, req)
}
