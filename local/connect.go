package local

import (
	"bufio"
	"crypto/tls"
	"io"
	"log"
	"net"
	"net/http"
	"net/url"
	"path"

	"h12.me/egress/protocol"
	"h12.me/errors"
)

type connector interface {
	connect(r *http.Request, cli net.Conn) error
}

func newConnector(typ string, remote *url.URL, fetcher fetcher, blockList *blockList, workDir string) (connector, error) {
	connectRemote := *remote
	connectRemote.Path = path.Join(connectRemote.Path, "/c")
	switch typ {
	case "direct":
		log.Print("connect DIRECTLY only!")
		return &directConnector{}, nil
	case "remote":
		log.Print("connect to REMOTE only!")
		return &remoteConnector{&connectRemote}, nil
	case "smart":
		return newSmartConnector(&connectRemote, blockList), nil
	case "faketls":
		log.Print("connect with FAKE TLS connector!")
		certs, err := newCertPool(path.Join(workDir, "cert"))
		if err != nil {
			return nil, err
		}
		return &fakeTLSConnector{
			fetcher: fetcher,
			certs:   certs}, nil
	}
	return nil, errors.Format("wrong connector type: %s", typ)
}

type directConnector struct{}

func (c *directConnector) connect(r *http.Request, cli net.Conn) error {
	return protocol.Connect(r.URL.Host, cli)
}

type remoteConnector struct {
	remote *url.URL
}

func (c *remoteConnector) connect(r *http.Request, cli net.Conn) error {
	remote, err := net.Dial("tcp", c.remote.Host)
	if err != nil {
		return errors.Wrap(err)
	}
	if c.remote.Scheme == "https" {
		remote = tls.Client(remote, &tls.Config{ServerName: "a"}) //???
	}
	defer remote.Close()

	if err := (&http.Request{
		Method: "CONNECT",
		URL:    c.remote,
		Header: http.Header{"Connect-Host": []string{r.URL.Host}},
	}).Write(remote); err != nil {
		return errors.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(remote), r)
	if err != nil {
		return errors.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		return errors.Format("error response from remote: %v", resp.StatusCode)
	}
	if err := protocol.OK200(cli); err != nil {
		return err
	}
	return protocol.Bind(cli, remote)
}

type smartConnector struct {
	direct directConnector
	remote remoteConnector
	list   *blockList
}

func newSmartConnector(remote *url.URL, blockList *blockList) *smartConnector {
	return &smartConnector{
		directConnector{},
		remoteConnector{remote},
		blockList,
	}
}

func (c *smartConnector) connect(r *http.Request, cli net.Conn) error {
	host := trimPort(r.URL.Host)
	if c.list.has(host) {
		return c.remote.connect(r, cli)
	}
	if err := c.direct.connect(r, cli); err == nil {
		return nil
	}
	if err := c.remote.connect(r, cli); err != nil {
		return err
	}
	if err := c.list.add(host); err != nil {
		log.Printf("fail to write list file: %s", err.Error())
	}
	return nil
}

type fakeTLSConnector struct {
	fetcher
	certs *certPool
}

func (f *fakeTLSConnector) connect(r *http.Request, cli net.Conn) error {
	host := r.URL.Host
	if err := protocol.OK200(cli); err != nil {
		return err
	}
	conn, err := fakeSecureConn(cli, trimPort(host), f.certs)
	if err != nil {
		return err
	}
	defer conn.Close()

	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		if !isEOF(err) {
			return errors.Wrap(err)
		}
		return nil
	}
	req.URL.Scheme = "https" // fill empty scheme with https
	req.URL.Host = req.Host  // fill empty Host with req.Host

	resp, err := f.fetch(req)
	if err != nil {
		protocol.Timeout504(conn)
		return err
	}
	defer resp.Body.Close()

	err = resp.Write(conn)
	if err != nil {
		switch err.(type) {
		case *net.OpError:
			return nil
		}
		if isEOF(err) {
			return nil
		}
		return errors.Wrap(err)
	}
	return nil
}
func trimPort(hostPort string) string {
	host, _, _ := net.SplitHostPort(hostPort)
	return host
}

func isEOF(err error) bool {
	return err == io.EOF || err.Error() == "EOF" || err.Error() == "unexpected EOF"
}
