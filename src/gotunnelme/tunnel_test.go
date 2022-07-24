package gotunnelme

import (
	"fmt"
	"testing"
	"time"
)

func TestTunnel(t *testing.T) {
	Debug = true
	host := "http://localtunnel.me"
	tunnel := NewTunnel(host)
	url, getUrlErr := tunnel.GetUrl("")
	if getUrlErr != nil {
		t.Fatal(getUrlErr)
	}
	fmt.Println("Get Url:", url)
	go func() {
		tunnelErr := tunnel.CreateTunnel(8787)
		if tunnelErr != nil {
			t.Fatal(tunnelErr)
		}
	}()
	time.Sleep(30 * time.Second)
	tunnel.StopTunnel()
}
