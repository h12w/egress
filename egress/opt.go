package main

import (
	"flag"
	"os"
	"path"
)

type option struct {
	Port    string
	Remote  string
	Dir     string
	Fetch   string
	Connect string
}

func (self *option) parse() {
	flag.StringVar(&self.Port, "port", "1984", "listening on http://localhost:<port>")
	flag.StringVar(&self.Remote, "remote", "http://127.0.0.1:8080", "remote address")
	flag.StringVar(&self.Dir, "dir", path.Join("$HOME", ".egress"), "directory for configuration file")
	flag.StringVar(&self.Fetch, "fetch", "smart", "fetcher: smart, remote and direct.")
	flag.StringVar(&self.Connect, "connect", "smart", "connector: smart, direct, remote or faketls.")
	flag.Parse()
	self.Dir = os.ExpandEnv(self.Dir)
}
