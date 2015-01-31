package local

import (
	"bufio"
	"bytes"
	"io"
	"io/ioutil"
	"log"
	"net"
	"net/http"
	"os"
	"sync"

	"h12.me/egress/geoip"
	"h12.me/egress/protocol"
	"h12.me/errors"
)

type fetcher interface {
	fetch(req *http.Request) (*http.Response, error)
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
	log.Printf("fetching %s", req.URL.String())
	req, err := protocol.MarshalRequest(req, g.remote)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	if resp.StatusCode != http.StatusOK {
		resp.Body.Close()
		return resp, nil
	}
	r, err := protocol.UnmarshalResponse(resp.Body, req)
	if r != nil && r.Body != nil {
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
type blockList struct {
	m    map[string]struct{}
	file string
	mu   sync.Mutex
}

// to sort the block list file:
//     rev blocklist | sort | rev

func newSmartFetcher(client *http.Client, remote, listFile string) (*smartFetcher, error) {
	list, err := newBlockList(listFile)
	if err != nil {
		return nil, err
	}
	return &smartFetcher{
		&directFetcher{client},
		&remoteFetcher{client, remote},
		list,
	}, nil
}
func newBlockList(listFile string) (*blockList, error) {
	m := make(map[string]struct{})
	f, err := os.OpenFile(listFile, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m[scanner.Text()] = struct{}{}
	}
	if scanner.Err() != nil {
		return nil, errors.Wrap(scanner.Err())
	}
	return &blockList{m: m, file: listFile}, nil
}

func (l *blockList) has(host string) bool {
	l.mu.Lock()
	defer l.mu.Unlock()
	_, ok := l.m[host]
	return ok
}

func (l *blockList) add(host string) error {
	l.mu.Lock()
	defer l.mu.Unlock()
	if _, ok := l.m[host]; !ok {
		l.m[host] = struct{}{}
		f, err := os.OpenFile(l.file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModeAppend)
		if err != nil {
			return errors.Wrap(err)
		}
		defer f.Close()
		f.WriteString(host)
		f.Write([]byte{'\n'})
	}
	return nil
}

func (d *smartFetcher) fetch(req *http.Request) (*http.Response, error) {
	if d.list.has(req.Host) {
		return d.remote.fetch(req)
	}
	rec := newBodyRecorder(req.Body)
	req.Body = rec
	if resp, err := d.direct.fetch(req); err == nil {
		return resp, err
	}
	req.Body = rec.reborn()
	resp, err := d.remote.fetch(req)
	if err != nil {
		return nil, err
	}
	if len(rec.data) > 0 {
		log.Print("REBORN SUCCESS!")
	}
	if ip := lookupIP(req.Host); ip != nil && geoip.ChinaList.Contains(ip) {
		log.Printf("Host %s in China, fetch remotely but not added", req.Host)
		return resp, nil
	}
	log.Printf("ADD HOST: %s", req.Host)
	if err := d.list.add(req.Host); err != nil {
		log.Printf("fail to write list file: %s", err.Error())
	}
	return resp, nil
}
func lookupIP(host string) net.IP {
	if addrs, err := net.LookupIP(host); err == nil {
		for _, ip := range addrs {
			ip = ip.To4()
			if ip != nil {
				return ip // return first IPv4 address
			}
		}
	}
	// ignore error
	return nil
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

func (b *bodyRecorder) reborn() io.ReadCloser {
	ioutil.ReadAll(b.rc)
	return ioutil.NopCloser(bytes.NewReader(b.data))
}
