package geoip_test

import (
	"bytes"
	"encoding/csv"
	"fmt"
	"io"
	"net"
	"os"
	"sort"
	"testing"
)

func TestExtract(*testing.T) {
	//extract()
}

func extract() {
	cnList := readAll(func(country string) bool {
		return country == "CN"
	})
	fmt.Println("size:", len(cnList))
	fmt.Println("sorted:", sort.IsSorted(cnList))
	f, _ := os.Create("china.go")
	fmt.Fprintln(f, "package geoip")
	fmt.Fprintln(f, "var ChinaList = IPNetListV4{")
	for _, rec := range cnList {
		lo, hi := rec.lo(), rec.hi()
		fmt.Fprintf(f, "{%d,%d}, // %15s - %s\n", ip4ToNum(lo), ip4ToNum(hi), lo.String(), hi.String())
	}
	fmt.Fprintln(f, "}")
	f.Close()
}

func readCountryMap() map[string]string {
	f, _ := os.Open("data/GeoLite2-Country-Locations-en.csv")
	r := csv.NewReader(f)
	r.Read() // read column names
	m := make(map[string]string)
	for {
		record, err := r.Read()
		if err == io.EOF {
			break
		}
		countryID := record[0]
		country := record[4]
		m[countryID] = country
	}
	f.Close()
	return m
}

func readAll(filter func(country string) bool) (records Records) {
	cmap := readCountryMap()
	f, _ := os.Open("data/GeoLite2-Country-Blocks-IPv4.csv")
	r := csv.NewReader(f)
	r.Read() // read column names
	for {
		csvRec, err := r.Read()
		if err == io.EOF {
			break
		}
		if filter == nil || filter(cmap[csvRec[1]]) {
			_, ipNet, _ := net.ParseCIDR(csvRec[0])
			record := Record{
				IPNet:   *ipNet,
				Country: cmap[csvRec[1]],
			}
			records = append(records, record)
		}
	}
	f.Close()
	return
}

func (rs Records) Filter(cond func(r *Record) bool) (records Records) {
	for i := range rs {
		if cond(&rs[i]) {
			records = append(records, rs[i])
		}
	}
	return
}

type (
	Record struct {
		IPNet   net.IPNet
		Country string
	}
	Records []Record
)

func (r Record) lo() net.IP {
	return r.IPNet.IP
}

func ip4ToNum(ip []byte) uint32 {
	return (uint32(ip[0]) << 24) |
		(uint32(ip[1]) << 16) |
		(uint32(ip[2]) << 8) |
		(uint32(ip[3]))
}

func (r Record) hi() net.IP {
	ip := make(net.IP, len(r.IPNet.IP))
	for i := range ip {
		ip[i] = r.IPNet.IP[i] | ^r.IPNet.Mask[i]
	}
	return ip
}

func (a Records) Len() int           { return len(a) }
func (a Records) Swap(i, j int)      { a[i], a[j] = a[j], a[i] }
func (a Records) Less(i, j int) bool { return bytes.Compare(a[i].IPNet.IP, a[j].IPNet.IP) < 0 }
