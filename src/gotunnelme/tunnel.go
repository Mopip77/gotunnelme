package gotunnelme

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"os"
	"strings"
)

var Debug = false

type TunnelConn struct {
	remoteHost   string
	remotePort   int
	localPort    int
	remoteConn   net.Conn
	localConn    net.Conn
	errorChannel chan error
}

func NewTunnelConn(remoteHost string, remotePort, localPort int) *TunnelConn {
	tunnelConn := &TunnelConn{}
	tunnelConn.remoteHost = remoteHost
	tunnelConn.remotePort = remotePort
	tunnelConn.localPort = localPort
	return tunnelConn
}

func (_self *TunnelConn) Tunnel(replyCh chan<- int) error {
	_self.errorChannel = make(chan error, 1) // clear previous channel's message
	remoteConn, remoteErr := _self.connectRemote()
	if remoteErr != nil {
		if Debug {
			fmt.Printf("Connect remote error[%s]!\n", remoteErr.Error())
		}
		replyCh <- -1
		return remoteErr
	}

	if Debug {
		fmt.Printf("Connect remote[%s:%d] successful!\n", _self.remoteHost, _self.remotePort)
	}

	localConn, localErr := _self.connectLocal()
	if localErr != nil {
		if Debug {
			fmt.Printf("Connect local error[%s]!\n", localErr.Error())
		}
		replyCh <- -1
		return localErr
	}
	if Debug {
		fmt.Printf("Connect local[:%d] successful!\n", _self.localPort)
	}

	_self.remoteConn = remoteConn
	_self.localConn = localConn
	go func() {
		var err error
		for {
			_, err = io.Copy(remoteConn, localConn)
			if err != nil {
				if Debug {
					fmt.Printf("Stop copy form local to remote! error=[%v]\n", err)
				}
				break
			}
		}
		_self.errorChannel <- err
	}()
	go func() {
		var err error
		for {
			_, err = io.Copy(localConn, remoteConn)
			if err != nil {
				if Debug {
					fmt.Printf("Stop copy form remote to local! error=[%v]\n", err)
				}
				break
			}

		}
		_self.errorChannel <- err
	}()
	err := <-_self.errorChannel
	replyCh <- 1
	return err
}

func (_self *TunnelConn) StopTunnel() error {
	if _self.remoteConn != nil {
		_self.remoteConn.Close()
	}
	if _self.localConn != nil {
		_self.localConn.Close()
	}
	return nil
}

func (_self *TunnelConn) connectRemote() (net.Conn, error) {
	remoteAddr := fmt.Sprintf("%s:%d", _self.remoteHost, _self.remotePort)
	addr := remoteAddr
	proxy := os.Getenv("HTTP_PROXY")
	if proxy == "" {
		proxy = os.Getenv("http_proxy")
	}
	if len(proxy) > 0 {
		url, err := url.Parse(proxy)
		if err == nil {
			addr = url.Host
		}
	}
	remoteConn, remoteErr := net.Dial("tcp", addr)
	if remoteErr != nil {
		return nil, remoteErr
	}

	if len(proxy) > 0 {
		fmt.Fprintf(remoteConn, "CONNECT %s HTTP/1.1\r\n", remoteAddr)
		fmt.Fprintf(remoteConn, "Host: %s\r\n", remoteAddr)
		fmt.Fprintf(remoteConn, "\r\n")
		br := bufio.NewReader(remoteConn)
		req, _ := http.NewRequest("CONNECT", remoteAddr, nil)
		resp, readRespErr := http.ReadResponse(br, req)
		if readRespErr != nil {
			return nil, readRespErr
		}
		if resp.StatusCode != 200 {
			f := strings.SplitN(resp.Status, " ", 2)
			return nil, errors.New(f[1])
		}

		if Debug {
			fmt.Printf("Connect %s by proxy[%s].\n", remoteAddr, proxy)
		}
	}
	return remoteConn, nil
}

func (_self *TunnelConn) connectLocal() (net.Conn, error) {
	localAddr := fmt.Sprintf("%s:%d", "localhost", _self.localPort)
	return net.Dial("tcp", localAddr)
}

type TunnelCommand int

const (
	stopTunnelCmd TunnelCommand = 1
)

type Tunnel struct {
	remoteHost      string
	assignedUrlInfo *AssignedUrlInfo
	localPort       int
	tunnelConns     []*TunnelConn
	cmdChan         chan TunnelCommand
}

func NewTunnel(remoteHost string) *Tunnel {
	tunnel := &Tunnel{}
	tunnel.remoteHost = remoteHost
	tunnel.cmdChan = make(chan TunnelCommand, 1)
	return tunnel
}

func (_self *Tunnel) startTunnel() error {
	if err := _self.checkLocalPort(); err != nil {
		return err
	}
	url, parseErr := url.Parse(_self.remoteHost)
	if parseErr != nil {
		return parseErr
	}
	replyCh := make(chan int, _self.assignedUrlInfo.MaxConnCount)
	remoteHost := url.Host
	for i := 0; i < _self.assignedUrlInfo.MaxConnCount; i++ {
		tunnelConn := NewTunnelConn(remoteHost, _self.assignedUrlInfo.Port, _self.localPort)
		_self.tunnelConns[i] = tunnelConn
		go tunnelConn.Tunnel(replyCh)
	}
L:
	for i := 0; i < _self.assignedUrlInfo.MaxConnCount; i++ {
		select {
		case <-replyCh:
		case cmd := <-_self.cmdChan:
			switch cmd {
			case stopTunnelCmd:
				break L
			}
		}
	}

	return nil
}

func (_self *Tunnel) checkLocalPort() error {
	localAddr := fmt.Sprintf("%s:%d", "localhost", _self.localPort)
	c, err := net.Dial("tcp", localAddr)
	if err != nil {
		return errors.New("can't connect local port!")
	}
	c.Close()
	return nil
}

func (_self *Tunnel) StopTunnel() {
	if Debug {
		fmt.Printf("Stop tunnel for localPort[%d]!\n", _self.localPort)
	}
	_self.cmdChan <- stopTunnelCmd
	for _, tunnelCon := range _self.tunnelConns {
		tunnelCon.StopTunnel()
	}
}

func (_self *Tunnel) GetUrl(assignedDomain string) (string, error) {
	if len(assignedDomain) == 0 {
		assignedDomain = "?new"
	}
	assignedUrlInfo, err := GetAssignedUrl(_self.remoteHost, assignedDomain)
	if err != nil {
		return "", err
	}
	_self.assignedUrlInfo = assignedUrlInfo
	_self.tunnelConns = make([]*TunnelConn, assignedUrlInfo.MaxConnCount)
	return assignedUrlInfo.Url, nil
}

func (_self *Tunnel) CreateTunnel(localPort int) error {
	_self.localPort = localPort
	return _self.startTunnel()
}
