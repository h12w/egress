// +build !appengine

package remote

import (
	"net/http"

	"h12.me/egress/protocol"
)

func ServeConnect(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(r)
	host := r.Header.Get("Connect-Host")
	if r.Method != "CONNECT" || host == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	cli, err := protocol.Hijack(w)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		ctx.Errorf("fail to hijack incoming connection")
		return
	}
	defer cli.Close()
	if err := protocol.Connect(host, cli); err != nil {
		ctx.Errorf("%v", err)
	}
}
