package log

import (
	"context"
	"regexp"
	"sync"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"k8s.io/apimachinery/pkg/types"
)

func TestStreamSetAdd(t *testing.T) {
	t.Parallel()

	instance := newStreamSet()

	first, ok := instance.add(context.Background(), types.UID("pod-1"), "app")
	require.True(t, ok)
	require.NotNil(t, first)

	_, ok = instance.add(context.Background(), types.UID("pod-1"), "app")
	assert.False(t, ok, "the same container must not be streamed twice")

	second, ok := instance.add(context.Background(), types.UID("pod-1"), "sidecar")
	require.True(t, ok, "each container of a pod has its own stream")
	require.NotNil(t, second)

	_, ok = instance.add(context.Background(), types.UID("pod-2"), "app")
	assert.True(t, ok)
}

func TestStreamSetCancelPod(t *testing.T) {
	t.Parallel()

	instance := newStreamSet()

	appCtx, ok := instance.add(context.Background(), types.UID("pod-1"), "app")
	require.True(t, ok)

	sidecarCtx, ok := instance.add(context.Background(), types.UID("pod-1"), "sidecar")
	require.True(t, ok)

	otherCtx, ok := instance.add(context.Background(), types.UID("pod-2"), "app")
	require.True(t, ok)

	assert.True(t, instance.cancelPod(types.UID("pod-1")), "streams were running")

	assert.Error(t, appCtx.Err(), "every container of the pod is cancelled")
	assert.Error(t, sidecarCtx.Err())
	assert.NoError(t, otherCtx.Err(), "other pods are untouched")

	assert.False(t, instance.cancelPod(types.UID("pod-1")), "nothing is left to cancel")

	_, ok = instance.add(context.Background(), types.UID("pod-1"), "app")
	assert.True(t, ok, "a cancelled stream can be opened again")
}

func TestStreamSetRemove(t *testing.T) {
	t.Parallel()

	instance := newStreamSet()

	streamCtx, ok := instance.add(context.Background(), types.UID("pod-1"), "app")
	require.True(t, ok)

	instance.remove(types.UID("pod-1"), "app")

	assert.Error(t, streamCtx.Err(), "removing cancels the stream")

	assert.NotPanics(t, func() {
		instance.remove(types.UID("pod-1"), "app")
	}, "removing twice is a no-op")
}

func TestStreamSetTerminal(t *testing.T) {
	t.Parallel()

	instance := newStreamSet()

	assert.False(t, instance.isTerminal(types.UID("pod-1")))
	assert.True(t, instance.markTerminal(types.UID("pod-1")), "first terminal event")
	assert.False(t, instance.markTerminal(types.UID("pod-1")), "every other terminal event")
	assert.True(t, instance.isTerminal(types.UID("pod-1")))
	assert.False(t, instance.isTerminal(types.UID("pod-2")))
}

func TestStreamSetConcurrently(t *testing.T) {
	t.Parallel()

	instance := newStreamSet()

	var wg sync.WaitGroup

	var added sync.Map

	for index := range 16 {
		wg.Go(func() {
			pod := types.UID("pod-" + string(rune('a'+index%4)))

			if _, ok := instance.add(context.Background(), pod, "app"); ok {
				added.Store(index, true)
			}

			instance.markTerminal(pod)
			instance.cancelPod(pod)
		})
	}

	wg.Wait()

	var count int

	added.Range(func(_, _ any) bool {
		count++

		return true
	})

	assert.LessOrEqual(t, count, 16)
	assert.NotZero(t, count)
}

func TestSinceSeconds(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		since time.Duration
		want  *int64
	}{
		"zero is not sent": {
			0,
			nil,
		},
		"negative is not sent": {
			-time.Hour,
			nil,
		},
		"one hour": {
			time.Hour,
			ptr(int64(3600)),
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			got := NewLogger("deployment", "app", nil, testCase.since).sinceSeconds()

			if testCase.want == nil {
				assert.Nil(t, got)

				return
			}

			require.NotNil(t, got)
			assert.Equal(t, *testCase.want, *got)
		})
	}
}

func TestGrepMatch(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		filters []string
		invert  bool
		text    string
		want    bool
	}{
		"no filter matches nothing": {
			nil,
			false,
			"hello",
			false,
		},
		"matching filter": {
			[]string{"hello"},
			false,
			"hello world",
			true,
		},
		"non matching filter": {
			[]string{"nope"},
			false,
			"hello world",
			false,
		},
		"any filter matches": {
			[]string{"nope", "world"},
			false,
			"hello world",
			true,
		},
		"inverted match is excluded": {
			[]string{"hello"},
			true,
			"hello world",
			false,
		},
		"inverted non match is kept": {
			[]string{"nope"},
			true,
			"hello world",
			true,
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			filters := make([]*regexp.Regexp, 0, len(testCase.filters))
			for _, filter := range testCase.filters {
				filters = append(filters, regexp.MustCompile(filter))
			}

			instance := NewLogger("deployment", "app", nil, time.Hour).
				WithLogRegexes(filters).
				WithInvertRegexp(testCase.invert)

			assert.Equal(t, testCase.want, instance.grepMatch(testCase.text))
		})
	}
}

func ptr[T any](value T) *T {
	return &value
}
