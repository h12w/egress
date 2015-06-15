package protocol // import "h12.me/egress/protocol"

import (
	"bufio"
	"bytes"
	"io"
	"net/http"

	"encoding/json"
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

type Header struct {
	Status        string
	StatusCode    int
	Proto         string
	ProtoMajor    int
	ProtoMinor    int
	Header        http.Header
	ContentLength int64
}

func MarshalResponse(resp *http.Response, w http.ResponseWriter) error {
	buf, err := json.Marshal(Header{
		Status:        resp.Status,
		StatusCode:    resp.StatusCode,
		Proto:         resp.Proto,
		ProtoMajor:    resp.ProtoMajor,
		ProtoMinor:    resp.ProtoMinor,
		Header:        resp.Header,
		ContentLength: resp.ContentLength,
	})
	if err != nil {
		return errors.Wrap(err)
	}
	w.Header().Set("egress-remote-header", string(buf))
	_, err = io.Copy(w, resp.Body)
	return errors.Wrap(err)
}

func UnmarshalResponse(resp *http.Response, req *http.Request) (*http.Response, error) {
	var h Header
	err := json.Unmarshal([]byte(resp.Header.Get("egress-remote-header")), &h)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	return &http.Response{
		Status:        h.Status,
		StatusCode:    h.StatusCode,
		Proto:         h.Proto,
		ProtoMajor:    h.ProtoMajor,
		ProtoMinor:    h.ProtoMinor,
		Header:        h.Header,
		ContentLength: h.ContentLength,
		Body:          resp.Body,
	}, nil
}

type nopReadCloser struct {
	io.Reader
}

func (nopReadCloser) Close() error { return nil }

type nopWriteCloser struct {
	io.Writer
}

func (nopWriteCloser) Close() error { return nil }
