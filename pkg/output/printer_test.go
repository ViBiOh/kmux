package output

import (
	"bytes"
	"testing"

	"github.com/stretchr/testify/assert"
)

func TestPrinter(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		events     []event
		wantStdout string
		wantStderr string
	}{
		"empty": {
			nil,
			"",
			"",
		},
		"std goes to stdout": {
			[]event{{std: true, message: "hello"}},
			"hello\n",
			"",
		},
		"err goes to stderr": {
			[]event{{message: "boom"}},
			"",
			"boom\n",
		},
		"prefix is written on stderr": {
			[]event{{std: true, prefix: "[ctx] ", message: "hello"}},
			"hello\n",
			"[ctx] ",
		},
		"one prefix per line": {
			[]event{{std: true, prefix: "[ctx] ", message: "first\nsecond"}},
			"first\nsecond\n",
			"[ctx] [ctx] ",
		},
		"trailing newline is not doubled": {
			[]event{{std: true, message: "hello\n"}},
			"hello\n",
			"",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			var stdout, stderr bytes.Buffer

			instance := newPrinter(&stdout, &stderr)

			go instance.start()

			for _, outputEvent := range testCase.events {
				instance.send(outputEvent)
			}

			instance.close()
			<-instance.done

			assert.Equal(t, testCase.wantStdout, stdout.String())
			assert.Equal(t, testCase.wantStderr, stderr.String())
		})
	}
}

func TestPrinterCloseIsIdempotent(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	instance := newPrinter(&stdout, &stderr)

	go instance.start()

	instance.send(event{std: true, message: "kept"})

	instance.close()
	<-instance.done

	assert.NotPanics(t, func() {
		instance.close()
		instance.send(event{std: true, message: "dropped"})
	}, "closing twice and sending afterwards must not panic")

	assert.Equal(t, "kept\n", stdout.String())
}

func TestPrinterConcurrentSend(t *testing.T) {
	t.Parallel()

	var stdout, stderr bytes.Buffer

	instance := newPrinter(&stdout, &stderr)

	go instance.start()

	done := make(chan struct{})

	for range 8 {
		go func() {
			defer func() { done <- struct{}{} }()

			for range 32 {
				instance.send(event{std: true, message: "line"})
			}
		}()
	}

	for range 8 {
		<-done
	}

	instance.close()
	<-instance.done

	assert.Equal(t, 8*32, bytes.Count(stdout.Bytes(), []byte("line\n")))
}

func TestOutputterChild(t *testing.T) {
	t.Parallel()

	cases := map[string]struct {
		name     string
		noPrefix bool
		prefix   string
		want     string
	}{
		"no name": {
			"",
			false,
			"",
			"",
		},
		"raw output drops everything": {
			"cluster",
			true,
			"[pod/container]",
			"",
		},
		"prefixes are appended": {
			"",
			false,
			"[pod/container]",
			"[pod/container] ",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			got := NewOutputter(testCase.name).Child(testCase.noPrefix, testCase.prefix)

			assert.Equal(t, testCase.want, got.prefix)
		})
	}
}
