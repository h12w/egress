package local

import (
	"net/http"

	"h12.me/egress/protocol"
)

type directFetcher struct {
	client *http.Client
}

func (d directFetcher) fetch(req *http.Request) (*http.Response, error) {
	return d.client.Transport.RoundTrip(req)
}

type gaeFetcher struct {
	client *http.Client
	remote string
}

func (g gaeFetcher) fetch(req *http.Request) (*http.Response, error) {
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

type dualFetcher struct {
	direct directFetcher
	remote fetcher
}

func (d dualFetcher) fetch(req *http.Request) (*http.Response, error) {
	if resp, err := d.direct.fetch(req); err == nil {
		return resp, err
	}
	return d.remote.fetch(req)
}
