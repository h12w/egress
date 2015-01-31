package protocol // import "h12.me/egress/protocol"

import (
	"bufio"
	"bytes"
	"io"
	"net/http"

	"h12.me/errors"
)

var (
	NewWriter = func(w io.Writer) io.WriteCloser { return nopWriteCloser{w} }
	NewReader = func(r io.Reader) io.ReadCloser { return nopReadCloser{r} }
)

func MarshalRequest(req *http.Request, remote string) (*http.Request, error) {
	var buf bytes.Buffer
	wc := NewWriter(&buf)
	if err := req.WriteProxy(wc); err != nil {
		wc.Close()
		return nil, errors.Wrap(err)
	}
	if err := wc.Close(); err != nil {
		return nil, errors.Wrap(err)
	}
	ret, err := http.NewRequest("POST", remote, &buf)
	return ret, errors.Wrap(err)
}

func UnmarshalRequest(req *http.Request) (*http.Request, error) {
	rc := NewReader(req.Body)
	ret, err := http.ReadRequest(bufio.NewReader(rc))
	if err != nil {
		rc.Close()
		return nil, errors.Wrap(err)
	}
	return ret, errors.Wrap(rc.Close())
}

func MarshalResponse(resp *http.Response, w io.Writer) error {
	wc := NewWriter(w)
	if err := resp.Write(wc); err != nil {
		wc.Close()
		return errors.Wrap(err)
	}
	return wc.Close()
}

func UnmarshalResponse(rd io.Reader, req *http.Request) (*http.Response, error) {
	rc := NewReader(rd)
	ret, err := http.ReadResponse(bufio.NewReader(rc), req)
	if err != nil {
		rc.Close()
		return nil, errors.Wrap(err)
	}
	return ret, errors.Wrap(rc.Close())
}

type nopReadCloser struct {
	io.Reader
}

func (nopReadCloser) Close() error { return nil }

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
