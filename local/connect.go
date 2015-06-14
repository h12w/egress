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
	connect(w http.ResponseWriter, host string) error
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

func (c *directConnector) connect(w http.ResponseWriter, host string) error {
	return protocol.Connect(w, host)
}

type remoteConnector struct {
	remote *url.URL
}

func (c *remoteConnector) connect(w http.ResponseWriter, host string) error {
	var remote net.Conn
	var err error
	switch c.remote.Scheme {
	case "https":
		host := setDefaultPort(c.remote.Host, "443")
		remote, err = tls.Dial("tcp", host, &tls.Config{InsecureSkipVerify: true})
	case "http":
		remote, err = net.Dial("tcp", setDefaultPort(c.remote.Host, "80"))
	default:
		return errors.Format("invalid scheme for the remote %s", c.remote.String())
	}
	if err != nil {
		return errors.Wrap(err)
	}
	defer remote.Close()

	if err := (&http.Request{
		Method: "GET",
		URL:    c.remote,
		Header: http.Header{
			"Connect-Host": []string{host},
			//			"Connection":   []string{"Keep-Alive"},
		},
	}).Write(remote); err != nil {
		return errors.Wrap(err)
	}
	resp, err := http.ReadResponse(bufio.NewReader(remote), nil)
	if err != nil {
		return errors.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		w.WriteHeader(resp.StatusCode)
		return errors.Format("error response from remote: %s", resp.Status)
	}
	cli, err := protocol.Hijack(w)
	if err != nil {
		return err
	}
	defer cli.Close()
	if err := protocol.OK200(cli); err != nil {
		return err
	}
	log.Print("binding")
	return protocol.Bind(cli, remote)
}
func setDefaultPort(hostPort, defaultPort string) string {
	host, port, _ := net.SplitHostPort(hostPort)
	if host == "" {
		host = hostPort
	}
	if port == "" {
		port = defaultPort
	}
	return net.JoinHostPort(host, port)
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

func (c *smartConnector) connect(w http.ResponseWriter, hostPort string) error {
	host := trimPort(hostPort)
	if c.list.has(host) {
		return c.remote.connect(w, hostPort)
	}
	if err := c.direct.connect(w, hostPort); err == nil {
		return nil
	}
	if err := c.remote.connect(w, hostPort); err != nil {
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

func (f *fakeTLSConnector) connect(w http.ResponseWriter, host string) error {
	cli, err := protocol.Hijack(w)
	if err != nil {
		return err
	}
	defer cli.Close()
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

	log.Printf("fetch start: %v", req.URL)
	resp, err := f.fetch(req)
	if err != nil {
		protocol.Timeout504(conn)
		return err
	}
	defer resp.Body.Close()
	log.Printf("fetch done: %v", req.URL)

	log.Printf("writing response: %v", req.URL)
	if err := resp.Write(conn); err != nil {
		log.Print(errors.Wrap(err), req.URL)
		switch err.(type) {
		case *net.OpError:
			return nil
		}
		if isEOF(err) {
			return nil
		}
		return errors.Wrap(err)
	}
	log.Printf("all done: %v", req.URL)
	return nil
}
func trimPort(hostPort string) string {
	host, _, _ := net.SplitHostPort(hostPort)
	return host
}

func isEOF(err error) bool {
	return err == io.EOF || err.Error() == "EOF" || err.Error() == "unexpected EOF"
}
