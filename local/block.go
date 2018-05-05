package local

import (
	"bufio"
	"log"
	"net"
	"os"
	"sync"

	"h12.io/egress/geoip"
	"h12.io/errors"
)

type blockList struct {
	m    map[string]struct{}
	file string
	mu   sync.Mutex
}

// to sort the block list file:
//     rev blocklist | sort | rev

func newBlockList(listFile string) (*blockList, error) {
	m := make(map[string]struct{})
	f, err := os.OpenFile(listFile, os.O_RDONLY|os.O_CREATE, 0600)
	if err != nil {
		if os.IsNotExist(err) {
			return &blockList{m: m, file: listFile}, nil
		}
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
	if ip := lookupIP(host); ip != nil && geoip.ChinaList.Contains(ip) {
		log.Printf("Host %s in China, fetch remotely but not added", host)
		return nil
	}
	log.Printf("ADD HOST: %s", host)

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
