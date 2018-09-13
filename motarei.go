package main

import (
	"context"
	"fmt"
	"log"
	"os"
	"runtime"
	"time"

	flags "github.com/jessevdk/go-flags"
	"github.com/kazeburo/motarei/discovery"
	"github.com/kazeburo/motarei/proxy"
)

// Version set in compile
var Version string

type cmdOpts struct {
	Listen              string        `short:"l" long:"listen" default:"0.0.0.0" description:"address to bind"`
	Port                string        `short:"p" long:"port" description:"Port number to bind" required:"true"`
	DockerLabel         string        `long:"docker-label" description:"label to filter container. eg app=nginx" required:"true"`
	DockerPrivatePort   uint16        `long:"docker-private-port" description:"Private port of container to use" required:"true"`
	ProxyConnectTimeout time.Duration `long:"proxy-connect-timeout" default:"60s" description:"timeout of connection to upstream"`
	Version             bool          `short:"v" long:"version" description:"Show version"`
}

func main() {
	opts := cmdOpts{}
	psr := flags.NewParser(&opts, flags.Default)
	_, err := psr.Parse()
	if err != nil {
		os.Exit(1)
	}

	if opts.Version {
		fmt.Printf(`motarei %s
Compiler: %s %s
`,
			Version,
			runtime.Compiler,
			runtime.Version())
		return

	}

	ctx := context.Background()

	d, err := discovery.NewDiscovery(opts.DockerLabel, opts.DockerPrivatePort)
	if err != nil {
		log.Fatalf("failed initialize discovery: %v", err)
	}
	_, err = d.Get(ctx)
	if err != nil {
		log.Fatalf("failed initialize discovery: %v", err)
	}
	go d.Run(ctx)

	p := proxy.NewProxy(opts.Listen, opts.Port, opts.ProxyConnectTimeout, d)
	err = p.Start(ctx)
	if err != nil {
		log.Fatalf("failed start proxy: %v", err)
	}
}
