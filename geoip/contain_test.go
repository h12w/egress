package geoip_test

import (
	"net"
	"testing"

	"h12.me/egress/geoip"
)

func TestSearch(t *testing.T) {
	for i, b := range []bool{
		!geoip.ChinaList.Contains(net.IP{0, 0, 0, 0}),
		!geoip.ChinaList.Contains(net.IP{1, 0, 0, 255}),

		geoip.ChinaList.Contains(net.IP{1, 0, 1, 0}),
		geoip.ChinaList.Contains(net.IP{1, 0, 1, 255}),

		geoip.ChinaList.Contains(net.IP{1, 0, 2, 0}),
		geoip.ChinaList.Contains(net.IP{1, 0, 3, 255}),

		!geoip.ChinaList.Contains(net.IP{1, 0, 4, 0}),
		!geoip.ChinaList.Contains(net.IP{1, 0, 7, 255}),

		geoip.ChinaList.Contains(net.IP{223, 255, 252, 0}),
		geoip.ChinaList.Contains(net.IP{223, 255, 253, 255}),

		!geoip.ChinaList.Contains(net.IP{223, 255, 254, 0}),
		!geoip.ChinaList.Contains(net.IP{255, 255, 255, 255}),

		// baidu.com
		geoip.ChinaList.Contains(net.IP{123, 125, 114, 144}),
		geoip.ChinaList.Contains(net.IP{220, 181, 57, 217}),

		// google.com
		!geoip.ChinaList.Contains(net.IP{195, 13, 189, 34}),
		!geoip.ChinaList.Contains(net.IP{74, 125, 227, 231}),

		// csdn.net
		geoip.ChinaList.Contains(net.IP{14, 17, 69, 22}),
	} {
		if !b {
			t.Fatalf("test case %d failed.", i)
		}
	}
}
