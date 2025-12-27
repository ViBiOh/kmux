package tcpool

import (
	"reflect"
	"testing"
)

func TestAdd(t *testing.T) {
	t.Parallel()

	type args struct {
		backend string
	}

	cases := map[string]struct {
		args args
		want []string
	}{
		"simple": {
			args{
				backend: "127.0.0.1:4000",
			},
			[]string{"127.0.0.1:4000"},
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			if got := New().Add(testCase.args.backend).backends; !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Add() = %#v, want %#v", got, testCase.want)
			}
		})
	}
}

func TestRemove(t *testing.T) {
	t.Parallel()

	type args struct {
		backend string
	}

	cases := map[string]struct {
		instance *Pool
		args     args
		want     []string
	}{
		"empty": {
			New(),
			args{
				backend: "127.0.0.1:4000",
			},
			nil,
		},
		"middle element": {
			New().Add("127.0.0.1:4000").Add("127.0.0.1:5000").Add("127.0.0.1:6000"),
			args{
				backend: "127.0.0.1:5000",
			},
			[]string{"127.0.0.1:4000", "127.0.0.1:6000"},
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			if got := testCase.instance.Remove(testCase.args.backend).backends; !reflect.DeepEqual(got, testCase.want) {
				t.Errorf("Remove() = %#v, want %#v", got, testCase.want)
			}
		})
	}
}

func TestNext(t *testing.T) {
	t.Parallel()

	loop := New().Add("127.0.0.1:4000").Add("127.0.0.1:5000")
	loop.next()

	cases := map[string]struct {
		instance *Pool
		want     string
	}{
		"empty": {
			New(),
			"",
		},
		"one item": {
			New().Add("127.0.0.1:4000"),
			"127.0.0.1:4000",
		},
		"many item": {
			New().Add("127.0.0.1:4000").Add("127.0.0.1:5000"),
			"127.0.0.1:5000",
		},
		"loop": {
			loop,
			"127.0.0.1:4000",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			if got := testCase.instance.next(); got != testCase.want {
				t.Errorf("Next() = `%s`, want `%s`", got, testCase.want)
			}
		})
	}
}
