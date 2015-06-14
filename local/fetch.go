package local

import (
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net/http"
	"net/url"
	"path"

	"h12.me/egress/protocol"
	"h12.me/errors"
)

type fetcher interface {
	fetch(req *http.Request) (*http.Response, error)
}

func newFetcher(typ string, remote *url.URL, httpClient *http.Client, blockList *blockList) (fetcher, error) {
	fetchRemote := *remote
	fetchRemote.Path = path.Join(fetchRemote.Path, "f")
	switch typ {
	case "direct":
		log.Print("fetch DIRECTLY only!")
		return &directFetcher{httpClient}, nil
	case "remote":
		log.Print("fetch from REMOTE only!")
		return &remoteFetcher{httpClient, fetchRemote.String()}, nil
	case "smart":
		return newSmartFetcher(httpClient, fetchRemote.String(), blockList)
	}
	return nil, errors.Format("wrong fetcher type: %s", typ)
}

type directFetcher struct {
	client *http.Client
}

func (d *directFetcher) fetch(req *http.Request) (*http.Response, error) {
	return d.client.Transport.RoundTrip(req)
}

type remoteFetcher struct {
	client *http.Client
	remote string
}

func (g *remoteFetcher) fetch(req *http.Request) (*http.Response, error) {
	log.Printf("fetch: %v", req.URL)
	req, err := protocol.MarshalRequest(req, g.remote)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		log.Printf("resp status: %v", resp.StatusCode)
		resp.Body.Close()
		return resp, nil
	}
	log.Print("unmarshaling body")
	r, err := protocol.UnmarshalResponse(resp.Body, req)
	if r != nil && r.Body != nil {
		log.Print("return body")
		r.Body = &chainCloser{r.Body, resp.Body}
	} else {
		resp.Body.Close()
	}
	return r, err
}

type chainCloser struct {
	io.ReadCloser
	c io.ReadCloser
}

func (c *chainCloser) Close() error {
	c.ReadCloser.Close()
	return c.c.Close()
}

type smartFetcher struct {
	direct *directFetcher
	remote fetcher
	list   *blockList
}

func newSmartFetcher(client *http.Client, remote string, blockList *blockList) (*smartFetcher, error) {
	return &smartFetcher{
		&directFetcher{client},
		&remoteFetcher{client, remote},
		blockList,
	}, nil
}

func (d *smartFetcher) fetch(req *http.Request) (*http.Response, error) {
	if d.list.has(req.Host) {
		return d.remote.fetch(req)
	}
	rec := newBodyRecorder(req.Body)
	req.Body = rec
	resp, err := d.direct.fetch(req)
	if err == nil {
		return resp, err
	}
	req.Body, err = rec.reborn()
	if err != nil {
		return nil, err
	}
	resp, err = d.remote.fetch(req)
	if err != nil {
		return nil, err
	}
	if len(rec.data) > 0 {
		log.Print("REBORN SUCCESS!")
	}
	if err := d.list.add(req.Host); err != nil {
		log.Printf("fail to write list file: %s", err.Error())
	}
	return resp, nil
}

type bodyRecorder struct {
	rc   io.ReadCloser
	data []byte
}

func newBodyRecorder(rc io.ReadCloser) *bodyRecorder {
	if rc == nil {
		return nil
	}
	return &bodyRecorder{rc: rc}
}

func (b *bodyRecorder) Read(p []byte) (n int, err error) {
	n, err = b.rc.Read(p)
	b.data = append(b.data, p[:n]...)
	return
}

func (b *bodyRecorder) Close() error {
	return b.rc.Close()
}

func (b *bodyRecorder) reborn() (io.ReadCloser, error) {
	if _, err := ioutil.ReadAll(b.rc); err != nil && err != io.EOF {
		return nil, err
	}
	return ioutil.NopCloser(bytes.NewReader(b.data)), nil
}
