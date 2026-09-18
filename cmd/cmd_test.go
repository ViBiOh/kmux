package cmd

import (
	"context"
	"testing"

	"github.com/ViBiOh/kmux/pkg/client"
	"github.com/ViBiOh/kmux/pkg/resource"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	appsv1 "k8s.io/api/apps/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/client-go/kubernetes/fake"
)

const testNamespace = "default"

func ptr[T any](value T) *T {
	return &value
}

func deployment(name, namespace string) runtime.Object {
	return &appsv1.Deployment{ObjectMeta: metav1.ObjectMeta{Name: name, Namespace: namespace}}
}

func TestParsePorts(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		rawPort    string
		wantLocal  uint64
		wantRemote string
		wantErr    string
	}{
		"single port": {
			"8080",
			8080, "8080", "",
		},
		"local and remote": {
			"8080:80",
			8080, "80", "",
		},
		"named remote port": {
			"8080:http",
			8080, "http", "",
		},
		"non numeric local port": {
			"http",
			0, "", "invalid local port",
		},
		"non numeric local port with a remote one": {
			"http:80",
			0, "", "invalid local port",
		},
		"empty remote port": {
			"8080:",
			0, "", "invalid remote port",
		},
		"out of range port": {
			"70000",
			0, "", "invalid local port",
		},
		"empty": {
			"",
			0, "", "invalid local port",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			local, remote, err := parsePorts(testCase.rawPort)

			if len(testCase.wantErr) != 0 {
				assert.ErrorContains(t, err, testCase.wantErr)

				return
			}

			require.NoError(t, err)
			assert.Equal(t, testCase.wantLocal, local)
			assert.Equal(t, testCase.wantRemote, remote)
		})
	}
}

func TestScaledReplicas(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		current int32
		factor  float64
		want    int32
	}{
		"double": {
			2, 2, 4,
		},
		"half rounds up": {
			3, 0.5, 2,
		},
		"fifty percent more": {
			2, 1.5, 3,
		},
		"same factor keeps the count": {
			3, 1, 3,
		},
		"down to zero": {
			5, 0, 0,
		},
		"from zero starts at one": {
			0, 1, 1,
		},
		"from zero doubles from one": {
			0, 2, 2,
		},
		"small factor keeps at least one": {
			1, 0.1, 1,
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			assert.Equal(t, testCase.want, scaledReplicas(testCase.current, testCase.factor))
		})
	}
}

// TestCompileContainerFilter is not parallel, it sets the shared container flag.
func TestCompileContainerFilter(t *testing.T) {
	cases := map[string]struct {
		container string
		wantNil   bool
		wantErr   string
	}{
		"no filter": {
			"", true, "",
		},
		"valid regexp": {
			"^app$", false, "",
		},
		"invalid regexp": {
			"[", true, "container filter compile",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			// not parallel, the container filter is a shared flag value
			container = testCase.container

			err := compileContainerFilter()

			if len(testCase.wantErr) != 0 {
				assert.ErrorContains(t, err, testCase.wantErr)
			} else {
				require.NoError(t, err)
			}

			assert.Equal(t, testCase.wantNil, containerRegexp == nil)
		})
	}

	container = ""
	containerRegexp = nil
}

func TestVersion(t *testing.T) {
	t.Parallel()

	assert.NotEmpty(t, version(), "a version is always printed")
}

// TestGetNamespace is not parallel, it sets the shared all-namespaces flag.
func TestGetNamespace(t *testing.T) {
	kube := client.New("test", "from-context", nil, nil)

	cases := map[string]struct {
		namespace    string
		allNamespace bool
		want         string
	}{
		"explicit namespace": {
			"asked", false, "asked",
		},
		"context namespace": {
			"", false, "from-context",
		},
		"all namespaces": {
			"", true, "",
		},
		"explicit namespace wins over all namespaces": {
			"asked", true, "asked",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			// not parallel, allNamespace is a shared flag value
			allNamespace = testCase.allNamespace
			defer func() { allNamespace = false }()

			assert.Equal(t, testCase.want, getNamespace(kube, testCase.namespace))
		})
	}
}

// TestListObjects is not parallel, it replaces the shared clients.
func TestListObjects(t *testing.T) {
	cases := map[string]struct {
		clients client.Array
		want    []string
	}{
		"no client": {
			nil,
			[]string{},
		},
		"single cluster": {
			client.Array{
				client.New("one", testNamespace, nil, fake.NewClientset(deployment("web", testNamespace), deployment("api", testNamespace))),
			},
			[]string{"api", "web"},
		},
		"only what every cluster has": {
			client.Array{
				client.New("one", testNamespace, nil, fake.NewClientset(deployment("web", testNamespace), deployment("api", testNamespace))),
				client.New("two", testNamespace, nil, fake.NewClientset(deployment("web", testNamespace), deployment("batch", testNamespace))),
			},
			[]string{"web"},
		},
		"nothing in common": {
			client.Array{
				client.New("one", testNamespace, nil, fake.NewClientset(deployment("web", testNamespace))),
				client.New("two", testNamespace, nil, fake.NewClientset(deployment("api", testNamespace))),
			},
			[]string{},
		},
		"a cluster without any object": {
			client.Array{
				client.New("one", testNamespace, nil, fake.NewClientset(deployment("web", testNamespace))),
				client.New("two", testNamespace, nil, fake.NewClientset()),
			},
			[]string{},
		},
	}

	lister, err := resource.ListerFor("deployments")
	require.NoError(t, err)

	previousClients := clients

	defer func() { clients = previousClients }()

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			clients = testCase.clients

			assert.Equal(t, testCase.want, listObjects(context.Background(), testNamespace, lister))
		})
	}
}
