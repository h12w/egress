package local

import (
	"fmt"
	"io"
	"net"
	"net/http"
	"sync"
)

type directDelegate struct {
	client *http.Client
}

func (d *directDelegate) tunnel(host string, cli net.Conn) error {
	srv, err := d.client.Transport.(*http.Transport).Dial("tcp", host)
	if err != nil {
		return fmt.Errorf("fail to dial directly: %s", err.Error())
	}
	defer srv.Close()
	if _, err := cli.Write([]byte("HTTP/1.1 200 OK\r\n\r\n")); err != nil {
		return err
	}
	return d.bind(cli, srv)
}

func (d *directDelegate) bind(cli, srv io.ReadWriter) error {
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

func (d *directDelegate) fetch(req *http.Request) (*http.Response, error) {
	return d.client.Transport.RoundTrip(req)
}
