package proxy

import (
	"context"
	"fmt"
	"io"
	"net"
	"time"

	"github.com/kazeburo/motarei/discovery"
	"go.uber.org/zap"
)

const (
	bufferSize = 0xFFFF
)

// Proxy proxy struct
type Proxy struct {
	listen  string
	port    uint16
	timeout time.Duration
	d       *discovery.Discovery
	done    chan struct{}
	logger  *zap.Logger
}

// NewProxy create new proxy
func NewProxy(listen string, port uint16, timeout time.Duration, d *discovery.Discovery, logger *zap.Logger) *Proxy {
	return &Proxy{
		listen:  listen,
		port:    port,
		timeout: timeout,
		d:       d,
		done:    make(chan struct{}),
		logger:  logger,
	}
}

// Start start new proxy
func (p *Proxy) Start(ctx context.Context) error {
	addr, err := net.ResolveTCPAddr("tcp", fmt.Sprintf("%s:%d", p.listen, p.port))
	if err != nil {
		return err
	}
	p.logger.Info("Start listen",
		zap.String("host", p.listen),
		zap.Uint16("port", p.port),
	)
	l, err := net.ListenTCP("tcp", addr)
	if err != nil {
		return err
	}

	go func() {
		select {
		case <-ctx.Done():
			p.logger.Info("Go shutdown",
				zap.String("host", p.listen),
				zap.Uint16("port", p.port),
			)
			l.Close()
		}
	}()

	for {
		conn, err := l.AcceptTCP()
		if err != nil {
			return err
		}
		conn.SetNoDelay(true)
		go p.handleConn(ctx, conn)
	}
}

func (p *Proxy) handleConn(ctx context.Context, c net.Conn) error {
	backends, err := p.d.Get(ctx, p.port)
	if err != nil {
		p.logger.Error("Failed to get backends", zap.Error(err))
		c.Close()
		return err
	}

	var s net.Conn
	for _, backend := range backends {
		// log.Printf("Proxy %s:%d => 127.0.0.1:%d (%s)", p.listen, p.port, backend.PublicPort, c.RemoteAddr())
		s, err = net.DialTimeout("tcp", fmt.Sprintf("127.0.0.1:%d", backend.PublicPort), p.timeout)
		if err == nil {
			break
		} else {
			p.logger.Error("Failed to connect backend", zap.Error(err))
		}
	}

	if err != nil {
		p.logger.Error("Giveup to connect backends", zap.Error(err))
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
				p.logger.Error("Copy from client", zap.Error(err))
				return
			}
		}
		return
	}()

	// upstream => client
	go func() {
		defer func() { doneCh <- true }()
		_, err := io.Copy(c, s)
		if err != nil {
			if !goClose {
				p.logger.Error("Copy from upstream", zap.Error(err))
				return
			}
		}
		return
	}()

	<-doneCh
	goClose = true
	s.Close()
	c.Close()
	<-doneCh
	return nil
}
