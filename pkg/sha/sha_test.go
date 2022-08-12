package sha

import "testing"

func TestNew(t *testing.T) {
	type args struct {
		o any
	}

	value := "test"

	cases := map[string]struct {
		args args
		want string
	}{
		"simple": {
			args{
				o: value,
			},
			"4d967a30111bf29f0eba01c448b375c1629b2fed01cdfcc3aed91f1b57d5dd5e",
		},
	}

	for intention, tc := range cases {
		t.Run(intention, func(t *testing.T) {
			if got := New(tc.args.o); got != tc.want {
				t.Errorf("New() = `%s`, want `%s`", got, tc.want)
			}
		})
	}
}

func BenchmarkNew(b *testing.B) {
	type testStruct struct {
		ID int
	}

	item := testStruct{}

	for i := 0; i < b.N; i++ {
		New(item)
	}
}
