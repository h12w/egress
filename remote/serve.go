package remote

import (
	"fmt"
	"net/http"

	"h12.me/egress/protocol"
)

func Serve(w http.ResponseWriter, r *http.Request) {
	ctx := NewContext(r)
	req, err := protocol.UnmarshalRequest(r)
	if err != nil {
		ctx.Errorf("fail to unmarshal a request: %s", err.Error())
		w.WriteHeader(http.StatusBadRequest)
		fmt.Fprint(w, err.Error())
		return
	}
	defer req.Body.Close()
	// a proxy should use Transport directly to avoid automatic redirection and
	// return the response as long as it is not nil.
	resp, err := ctx.NewClient().Transport.RoundTrip(req)
	if resp == nil {
		ctx.Errorf("fail to fetch: %v", err)
		w.WriteHeader(http.StatusGatewayTimeout)
		return
	}
	defer resp.Body.Close()
	if err := protocol.MarshalResponse(resp, w); err != nil {
		ctx.Errorf("fail to marshal a response: %s", err.Error())
		if hij, ok := w.(http.Hijacker); ok {
			if conn, _, err := hij.Hijack(); err != nil {
				conn.Close() // force closing
			}
		}
		return
	}
}
