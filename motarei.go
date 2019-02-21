package main

import (
	"context"
	"fmt"
	"os"
	"runtime"
	"time"

	flags "github.com/jessevdk/go-flags"
	"github.com/kazeburo/motarei/discovery"
	"github.com/kazeburo/motarei/proxy"
	"go.uber.org/zap"
	"golang.org/x/sync/errgroup"
)

// Version set in compile
var Version string

type cmdOpts struct {
	BindIP              string        `long:"bind" default:"0.0.0.0" description:"IP address to bind"`
	DockerLabel         string        `long:"label" short:"l" description:"label to filter container. eg app=nginx" required:"true"`
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

	logger, err := zap.NewProduction()
	if err != nil {
		os.Exit(1)
	}
	defer logger.Sync()

	ctx := context.Background()

	d, err := discovery.NewDiscovery(ctx, opts.DockerLabel, logger)
	if err != nil {
		logger.Fatal("failed initialize discovery", zap.Error(err))
	}
	privatePorts := d.GetPrivatePorts()
	_, err = d.RunDiscovery(ctx)
	if err != nil {
		logger.Fatal("failed first discovery", zap.Error(err))
	}
	go d.Run(ctx)

	eg, ctx := errgroup.WithContext(ctx)
	ctx, cancel := context.WithCancel(ctx)
	defer cancel()

	for _, port := range privatePorts {
		port := port
		eg.Go(func() error {
			p := proxy.NewProxy(opts.BindIP, port, opts.ProxyConnectTimeout, d, logger)
			return p.Start(ctx)
		})
	}
	if err := eg.Wait(); err != nil {
		defer cancel()
		logger.Fatal("failed to start proxy", zap.Error(err))
	}
}
