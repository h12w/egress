// +build !appengine

package remote

import (
	"net/http"

	"h12.io/egress/protocol"
)

func ServeConnect(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(r)
	host := r.Header.Get("Connect-Host")
	if host == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := protocol.Connect(w, host); err != nil {
		ctx.Errorf("%v", err)
	}
}
