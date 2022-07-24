package main

import (
	"flag"
	"fmt"

	"github.com/Mopip77/gotunnelme/src/gotunnelme"
)

var (
	port = 8080
	subDomain = ""
	host = "http://localtunnel.me"
)

func flagParse() {
	flag.IntVar(&port, "port", 8080, "local port")
	flag.StringVar(&subDomain, "subdomain", "", "subdomain")
	flag.StringVar(&host, "host", "http://localtunnel.me", "host")
	flag.Parse()
}

func main() {
	flagParse()
	fmt.Println("port:", port)
	fmt.Println("subdomain:", subDomain)
	fmt.Println("host:", host)
	fmt.Println()
	
	t := gotunnelme.NewTunnel(host)
	url, err := t.GetUrl(subDomain)
	if err != nil {
		panic(err)
	}
	print(url)
	err = t.CreateTunnel(port)
	if err != nil {
		panic(err)
	}
	t.StopTunnel()
}
