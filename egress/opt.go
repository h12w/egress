package main

import (
	"flag"
	"os"
	"path"
)

type option struct {
	Port   string
	Remote string
	Dir    string
}

func (self *option) parse() {
	flag.StringVar(&self.Port, "port", "9527", "listening on 0.0.0.0:<port>")
	flag.StringVar(&self.Remote, "remote", "", "remote address")
	flag.StringVar(&self.Dir, "dir", path.Join("$HOME", ".egress"), "directory for configuration file")
	flag.Parse()
	self.Dir = os.ExpandEnv(self.Dir)
}
