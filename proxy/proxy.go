package proxy

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"time"

	"github.com/kazeburo/motarei/discovery"
	ss "github.com/lestrrat/go-server-starter-listener"
)

const (
	bufferSize = 0xFFFF
)

// Proxy proxy struct
type Proxy struct {
	listen  string
	port    string
	timeout uint16
	d       *discovery.Discovery
	done    chan struct{}
}

// NewProxy create new proxy
func NewProxy(listen string, port string, timeout uint16, d *discovery.Discovery) *Proxy {
	return &Proxy{
		listen:  listen,
		port:    port,
		timeout: timeout,
		d:       d,
		done:    make(chan struct{}),
	}
}

// Start start new proxy
func (p *Proxy) Start(ctx context.Context) error {
	l, err := ss.NewListener()
	if l == nil || err != nil {
		l, err = net.Listen("tcp", fmt.Sprintf("%s:%s", p.listen, p.port))
		if err != nil {
			return err
		}
	}

	for {
		conn, err := l.Accept()
		if err != nil {
			return err
		}
		go p.handleConn(ctx, conn)
	}
}

func (p *Proxy) handleConn(ctx context.Context, c net.Conn) error {
	backends, err := p.d.Get(ctx)
	if err != nil {
		log.Printf("Failed to get backends: %v", err)
		c.Close()
		return err
	}
	var s net.Conn
	for _, backend := range backends {
		// todo timeout
		s, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", backend.PublicPort), time.Duration(p.timeout)*time.Second)
		if err == nil {
			break
		} else {
			log.Printf("Failed to connect backend: %v", err)
		}
	}
	if err != nil {
		log.Printf("Giveup to connect backends: %v", err)
		c.Close()
		return err
	}

	doneCh := make(chan bool)
	goClose := false

	// client => upstream
	go func() {
		defer func() { doneCh <- true }()
		_, err := io.Copy(s, c)
		if err != nil {
			if !goClose {
				log.Printf("Copy from client: %v", err)
			}
			return
		}
	}()

	// upstream => client
	go func() {
		defer func() { doneCh <- true }()
		_, err := io.Copy(c, s)
		if err != nil {
			if !goClose {
				log.Printf("Copy from upstream: %v", err)
			}
			return
		}
	}()

	<-doneCh
	goClose = true
	s.Close()
	c.Close()
	<-doneCh

	return nil
}
