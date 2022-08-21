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
				backend: "localhost:4000",
			},
			[]string{"localhost:4000"},
		},
	}

	for intention, testCase := range cases {
		intention, testCase := intention, testCase

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
				backend: "localhost:4000",
			},
			nil,
		},
		"middle element": {
			New().Add("localhost:4000").Add("localhost:5000").Add("localhost:6000"),
			args{
				backend: "localhost:5000",
			},
			[]string{"localhost:4000", "localhost:6000"},
		},
	}

	for intention, testCase := range cases {
		intention, testCase := intention, testCase

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

	loop := New().Add("localhost:4000").Add("localhost:5000")
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
			New().Add("localhost:4000"),
			"localhost:4000",
		},
		"many item": {
			New().Add("localhost:4000").Add("localhost:5000"),
			"localhost:5000",
		},
		"loop": {
			loop,
			"localhost:4000",
		},
	}

	for intention, testCase := range cases {
		intention, testCase := intention, testCase

		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			if got := testCase.instance.next(); got != testCase.want {
				t.Errorf("Next() = `%s`, want `%s`", got, testCase.want)
			}
		})
	}
}
