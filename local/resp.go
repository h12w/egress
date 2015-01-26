package local

import "io"

func ok200(w io.Writer) error {
	_, err := w.Write([]byte("HTTP/1.1 200 OK\r\n\r\n"))
	return err
}

func timeout504(w io.Writer) error {
	_, err := w.Write([]byte("HTTP/1.1 504 Gateway timeout\r\n\r\n"))
	return err
}
