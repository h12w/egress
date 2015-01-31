// +build !appengine

package remote

import (
	"log"
	"net"
	"net/http"
	"time"
)

type Context struct {
	req *http.Request
}

func NewContext(r *http.Request) *Context {
	return &Context{r}
}

func (ctx *Context) NewClient() *http.Client {
	return &http.Client{
		Transport: &http.Transport{
			Dial: (&net.Dialer{
				Timeout:   10 * time.Second,
				KeepAlive: 30 * time.Second,
			}).Dial,
			TLSHandshakeTimeout: 10 * time.Second,
		}}
}

func (ctx *Context) Errorf(format string, a ...interface{}) {
	log.Printf(format, a...)
}
