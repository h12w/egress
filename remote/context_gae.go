// +build appengine

package remote

import (
	"net/http"

	"appengine"
	"appengine/urlfetch"
)

type Context struct {
	req *http.Request
	appengine.Context
}

func NewContext(r *http.Request) *Context {
	return &Context{r, appengine.NewContext(r)}
}

func (ctx *Context) NewClient() *http.Client {
	return urlfetch.Client(ctx.Context)
}
