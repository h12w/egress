package protocol

import (
	"io"
	"net"
	"net/http"
	"sync"

	"h12.me/errors"
)

func Connect(host string, cli net.Conn) error {
	srv, err := net.Dial("tcp", host)
	if err != nil {
		Timeout504(cli)
		return errors.Wrap(err)
	}
	defer srv.Close()
	if err := OK200(cli); err != nil {
		return err
	}
	return Bind(cli, srv)
}

func Bind(cli, srv io.ReadWriter) error {
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

func OK200(w io.Writer) error {
	_, err := w.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	return errors.Wrap(err)
}

func Timeout504(w io.Writer) error {
	_, err := w.Write([]byte("HTTP/1.1 504 Gateway timeout\r\n\r\n"))
	return errors.Wrap(err)
}

func Hijack(w http.ResponseWriter) (net.Conn, error) {
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
