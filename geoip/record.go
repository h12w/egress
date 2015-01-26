package geoip

import "sort"

type (
	IPNetV4 struct {
		Lo uint32
		Hi uint32
	}
	IPNetListV4 []IPNetV4
)

func (a IPNetListV4) Contains(ip []byte) bool {
	ip4 := ip4ToNum(ip)
	i := sort.Search(len(a), func(i int) bool {
		return a[i].Hi >= ip4
	})
	if i < len(a) {
		return a[i].Lo <= ip4 && ip4 <= a[i].Hi
	}
	return false
}

func ip4ToNum(ip []byte) uint32 {
	return (uint32(ip[0]) << 24) |
		(uint32(ip[1]) << 16) |
		(uint32(ip[2]) << 8) |
		(uint32(ip[3]))
}
