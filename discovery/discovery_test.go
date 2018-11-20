package discovery

import (
	"context"
	"errors"
	"testing"

	"github.com/docker/docker/api/types"
)

type fakeClient struct {
	id         string
	status     string
	withHealth bool
	err        error
}

func (f *fakeClient) ContainerInspect(ctx context.Context, containerID string) (types.ContainerJSON, error) {
	var containerJSON types.ContainerJSON

	if f.id != containerID {
		return containerJSON, errors.New("different container id")
	}

	if f.err != nil {
		return containerJSON, f.err
	}

	var health *types.Health
	if f.withHealth {
		health = &types.Health{
			Status: f.status,
		}
	}
	containerJSON = types.ContainerJSON{
		ContainerJSONBase: &types.ContainerJSONBase{
			State: &types.ContainerState{
				Health: health,
			},
		},
	}

	return containerJSON, nil
}

func TestFilerHealthy(t *testing.T) {
	testcase := map[string]struct {
		client     inspector
		containers []types.Container
		expected   int
	}{
		"Healthy": {
			client: &fakeClient{
				id:         "test0",
				withHealth: true,
				status:     types.Healthy,
			},
			containers: []types.Container{
				types.Container{
					ID: "test0",
				},
			},
			expected: 1,
		},
		"Unhealthy": {
			client: &fakeClient{
				id:         "test0",
				withHealth: true,
				status:     types.Unhealthy,
			},
			containers: []types.Container{
				types.Container{
					ID: "test0",
				},
			},
			expected: 0,
		},
		"Starting": {
			client: &fakeClient{
				id:         "test0",
				withHealth: true,
				status:     types.Starting,
			},
			containers: []types.Container{
				types.Container{
					ID: "test0",
				},
			},
			expected: 0,
		},
		"No health check": {
			client: &fakeClient{
				id:         "test0",
				withHealth: false,
			},
			containers: []types.Container{
				types.Container{
					ID: "test0",
				},
			},
			expected: 1,
		},
		"error": {
			client: &fakeClient{
				id:         "test0",
				withHealth: true,
				status:     types.Healthy,
			},
			containers: []types.Container{
				types.Container{
					ID: "test0",
				},
				types.Container{
					ID: "test1",
				},
			},
			expected: 2,
		},
	}

	ctx := context.Background()
	for name, tc := range testcase {
		actual := len(filterHealthy(ctx, tc.client, tc.containers))
		if actual != tc.expected {
			t.Errorf("unexpected count: actual=%v, expected=%v in %s", actual, tc.expected, name)
		}
	}
}
