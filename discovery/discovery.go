package discovery

import (
	"context"
	"fmt"
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
	cli          *client.Client
	mu           *sync.Mutex
	backends     map[uint16][]BackendContainer
	filter       filters.Args
	privatePorts []uint16
}

// NewDiscovery : create new Discovery
func NewDiscovery(ctx context.Context, label string) (*Discovery, error) {
	cli, err := client.NewClientWithOpts(client.WithVersion("1.30"))
	if err != nil {
		return nil, err
	}
	filter := filters.NewArgs()
	filter.Add("label", label)

	containers, err := cli.ContainerList(ctx, types.ContainerListOptions{Filters: filter})
	if err != nil {
		return nil, err
	}
	if len(containers) < 1 {
		return nil, fmt.Errorf("Could not find containers with label: %s", label)
	}
	sort.SliceStable(containers, func(i, j int) bool { return containers[i].ID < containers[j].ID })
	sort.SliceStable(containers, func(i, j int) bool { return containers[i].Created > containers[j].Created })
	var privatePorts []uint16
	for _, port := range containers[0].Ports {
		if port.Type == "tcp" {
			privatePorts = append(privatePorts, port.PrivatePort)
		}
	}
	if len(privatePorts) < 1 {
		return nil, fmt.Errorf("containers were found, but that container has no public port. container-id:%s", containers[0].ID)
	}

	return &Discovery{
		cli:          cli,
		mu:           new(sync.Mutex),
		filter:       filter,
		privatePorts: privatePorts,
	}, nil
}

// GetPrivatePorts get private ports
func (d *Discovery) GetPrivatePorts() []uint16 {
	return d.privatePorts
}

// RunDiscovery: run Discovery
func (d *Discovery) RunDiscovery(ctx context.Context) (map[uint16][]BackendContainer, error) {
	containers, err := d.cli.ContainerList(ctx, types.ContainerListOptions{Filters: d.filter})
	if err != nil {
		return nil, err
	}

	backends := map[uint16][]BackendContainer{}
	for _, privatePort := range d.privatePorts {
		portBackends := []BackendContainer{}
		for _, container := range containers {
			publicPort := uint16(0)
			for _, port := range container.Ports {
				if port.Type == "tcp" && port.PrivatePort == privatePort {
					publicPort = port.PublicPort
				}
			}
			if publicPort > 0 {
				portBackends = append(portBackends, BackendContainer{container, publicPort})
			}
			sort.SliceStable(portBackends, func(i, j int) bool { return portBackends[i].C.ID < portBackends[j].C.ID })
			sort.SliceStable(portBackends, func(i, j int) bool { return portBackends[i].C.Created > portBackends[j].C.Created })
		}
		backends[privatePort] = portBackends
	}

	d.mu.Lock()
	defer d.mu.Unlock()
	d.backends = backends
	return d.backends, nil
}

// Get access token
func (d *Discovery) Get(ctx context.Context, privatePort uint16) ([]BackendContainer, error) {
	d.mu.Lock()
	portBackends, ok := d.backends[privatePort]
	d.mu.Unlock()

	if ok && len(portBackends) > 0 {
		return portBackends, nil
	}
	backends, err := d.RunDiscovery(ctx)
	if err != nil {
		return nil, err
	}
	portBackends, ok = backends[privatePort]
	if ok && len(portBackends) > 0 {
		return portBackends, nil
	}
	return nil, fmt.Errorf("Could not find Backends for private port: %d", privatePort)
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
			_, err := d.RunDiscovery(ctx)
			if err != nil {
				log.Printf("Regularly runDiscovery failed:%v", err)
			}
		}
	}
}
