package local

import (
	"bufio"
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
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
		return fmt.Errorf("fail to dial directly: %s", err.Error())
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
			errChan <- fmt.Errorf("fail to copy from client to server: %s", err.Error())
		}
	}()
	go func() {
		defer wg.Done()
		_, err := io.Copy(cli, srv)
		if err != nil {
			errChan <- fmt.Errorf("fail to copy from server to client: %s", err.Error())
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
	conn, err := fakeTLSHandeshake(cli, trimPort(host), g.certs)
	if err != nil {
		return fmt.Errorf("fail to fake handshake with the client: %s", err.Error())
	}
	defer conn.Close()

	req, err := http.ReadRequest(bufio.NewReader(conn))
	if err != nil {
		if err != io.EOF {
			return fmt.Errorf("ReadRequest: %s", err.Error())
		}
		return nil
	}
	req.URL.Scheme = "https" // fill empty scheme with https
	req.URL.Host = req.Host  // fill empty Host with req.Host

	resp, err := g.fetch(req)
	if err != nil {
		return fmt.Errorf("fail to fetch: %s", err.Error())
	}
	defer resp.Body.Close()

	err = resp.Write(conn)
	if err != nil {
		if err != io.EOF {
			return fmt.Errorf("Write: %s", err.Error())
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
	return err
}

func timeout504(w io.Writer) error {
	_, err := w.Write([]byte("HTTP/1.1 504 Gateway timeout\r\n\r\n"))
	return err
}
