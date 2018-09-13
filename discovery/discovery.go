package discovery

import (
	"context"
	"log"
	"sort"
	"sync"
	"time"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/client"
)

const (
	refreshInterval = 1
)

// BackendContainer backend container and port
type BackendContainer struct {
	C          types.Container
	PublicPort uint16
}

// Discovery backend discovery
type Discovery struct {
	cli         *client.Client
	mu          *sync.Mutex
	backends    []BackendContainer
	filter      filters.Args
	privatePort uint16
}

// NewDiscovery : create new Discovery
func NewDiscovery(label string, privatePort uint16) (*Discovery, error) {
	cli, err := client.NewClientWithOpts(client.WithVersion("1.30"))
	if err != nil {
		return nil, err
	}
	filter := filters.NewArgs()
	filter.Add("label", label)
	return &Discovery{
		cli:         cli,
		mu:          new(sync.Mutex),
		filter:      filter,
		privatePort: privatePort,
	}, nil
}

func (d *Discovery) runDiscovery(ctx context.Context) ([]BackendContainer, error) {
	containers, err := d.cli.ContainerList(ctx, types.ContainerListOptions{Filters: d.filter})
	if err != nil {
		return nil, err
	}

	backends := []BackendContainer{}
	for _, container := range containers {
		publicPort := uint16(0)
		for _, port := range container.Ports {
			if port.PrivatePort == d.privatePort {
				publicPort = port.PublicPort
			}
		}
		if publicPort > 0 {
			backends = append(backends, BackendContainer{container, publicPort})
		}
		sort.SliceStable(backends, func(i, j int) bool { return backends[i].C.ID < backends[j].C.ID })
		sort.SliceStable(backends, func(i, j int) bool { return backends[i].C.Created > backends[j].C.Created })
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.backends = backends
	return d.backends, nil
}

// Get access token
func (d *Discovery) Get(ctx context.Context) ([]BackendContainer, error) {
	d.mu.Lock()
	backends := d.backends
	d.mu.Unlock()

	if len(backends) > 0 {
		return backends, nil
	}
	backends, err := d.runDiscovery(ctx)
	if err != nil {
		return backends, err
	}
	return backends, nil
}

// Run refresh token regularly
func (d *Discovery) Run(ctx context.Context) {
	ticker := time.NewTicker(refreshInterval * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case _ = <-ticker.C:
			_, err := d.runDiscovery(ctx)
			if err != nil {
				log.Printf("Regularly runDiscovery failed:%v", err)
			}
		}
	}
}
