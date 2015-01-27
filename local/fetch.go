package local

import (
	"bufio"
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
	req, err := protocol.MarshalRequest(req, g.remote)
	if err != nil {
		return nil, err
	}
	resp, err := g.client.Do(req)
	if err != nil {
		return nil, errors.Wrap(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		return resp, nil
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
	if resp, err := d.direct.fetch(req); err == nil {
		return resp, err
	}
	resp, err := d.remote.fetch(req)
	if err != nil {
		return nil, err
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
