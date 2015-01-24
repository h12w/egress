// +build !appengine

package remote

import (
	"log"
	"net/http"
)

type Context struct {
	req *http.Request
}

func NewContext(r *http.Request) *Context {
	return &Context{r}
}

func (ctx *Context) NewClient() *http.Client {
	return &http.Client{}
}

func (ctx *Context) Errorf(format string, a ...interface{}) {
	log.Printf(format, a...)
}
