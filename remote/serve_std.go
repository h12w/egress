// +build !appengine

package remote

import (
	"net/http"
	"net/url"

	"h12.me/egress/protocol"
)

func ServeConnect(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(r)
	host := r.Header.Get("Connect-Host")
	if host == "" {
		w.WriteHeader(http.StatusBadRequest)
		return
	}
	if err := protocol.Connect(w, &http.Request{URL: &url.URL{Host: host}}); err != nil {
		ctx.Errorf("%v", err)
	}
}
