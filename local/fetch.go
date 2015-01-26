package local

import (
	"bufio"
	"fmt"
	"log"
	"net/http"
	"os"
	"sync"

	"h12.me/egress/protocol"
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

type gaeFetcher struct {
	client *http.Client
	remote string
}

func (g *gaeFetcher) fetch(req *http.Request) (*http.Response, error) {
	req, err := protocol.MarshalRequest(req, g.remote)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, err
	}
	// UnmarshalResponse will do resp.Body.Close
	return protocol.UnmarshalResponse(resp.Body, req)
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

func newSmartFetcher(client *http.Client, remote, listFile string) (*smartFetcher, error) {
	list, err := newBlockList(listFile)
	if err != nil {
		return nil, err
	}
	return &smartFetcher{
		&directFetcher{client},
		&gaeFetcher{client, remote},
		list,
	}, nil
}
func newBlockList(listFile string) (*blockList, error) {
	m := make(map[string]struct{})
	f, err := os.OpenFile(listFile, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		return nil, fmt.Errorf("fail to open block list file: %s", err.Error())
	}
	defer f.Close()
	scanner := bufio.NewScanner(f)
	for scanner.Scan() {
		m[scanner.Text()] = struct{}{}
	}
	if scanner.Err() != nil {
		return nil, fmt.Errorf("fail to read block list file: %s", scanner.Err())
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
		log.Printf("ADD HOST: %s", host)
		l.m[host] = struct{}{}
		f, err := os.OpenFile(l.file, os.O_WRONLY|os.O_CREATE|os.O_APPEND, os.ModeAppend)
		if err != nil {
			return err
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
	if resp, err := d.direct.fetch(req); err == nil {
		return resp, err
	}
	resp, err := d.remote.fetch(req)
	if err != nil {
		return nil, err
	}
	if err := d.list.add(req.Host); err != nil {
		log.Printf("fail to write list file: %s", err.Error())
	}
	return resp, nil
}
