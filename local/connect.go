package local

import (
	"bufio"
	"io"
	"net"
	"net/http"
	"sync"

	"h12.me/errors"
)

type connector interface {
	connect(host string, cli net.Conn) error
}

type directConnector struct {
	directFetcher
}

func (d *directConnector) connect(host string, cli net.Conn) error {
	srv, err := d.client.Transport.(*http.Transport).Dial("tcp", host)
	if err != nil {
		timeout504(cli)
		return errors.Wrap(err)
	}
	defer srv.Close()
	if err := ok200(cli); err != nil {
		return err
	}
	return d.bind(cli, srv)
}
func (d *directConnector) bind(cli, srv io.ReadWriter) error {
	var wg sync.WaitGroup
	errChan := make(chan error, 2)
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, err := io.Copy(srv, cli)
		if err != nil {
			errChan <- errors.Wrap(err)
		}
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(cli, srv)
		if err != nil {
			errChan <- errors.Wrap(err)
		}
	}()
	wg.Wait()
	if err, hasErr := <-errChan; hasErr {
		return err
	}
	return nil
}

type fakeTLSConnector struct {
	fetcher
	certs *certPool
}

func (g *fakeTLSConnector) connect(host string, cli net.Conn) error {
	if err := ok200(cli); err != nil {
		return err
	}
	conn, err := fakeSecureConn(cli, trimPort(host), g.certs)
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

	resp, err := g.fetch(req)
	if err != nil {
		timeout504(conn)
		return err
	}
	defer resp.Body.Close()

	err = resp.Write(conn)
	if err != nil {
		if !isEOF(err) {
			return errors.Wrap(err)
		}
		return nil
	}
	return nil
}
func trimPort(hostPort string) string {
	host, _, _ := net.SplitHostPort(hostPort)
	return host
}

func ok200(w io.Writer) error {
	_, err := w.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	return errors.Wrap(err)
}

func timeout504(w io.Writer) error {
	_, err := w.Write([]byte("HTTP/1.1 504 Gateway timeout\r\n\r\n"))
	return errors.Wrap(err)
}

func isEOF(err error) bool {
	return err == io.EOF || err.Error() == "EOF" || err.Error() == "unexpected EOF"
}
