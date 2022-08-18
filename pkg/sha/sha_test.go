package sha

import "testing"

func TestNew(t *testing.T) {
	type args struct {
		content any
	}

	cases := map[string]struct {
		args args
		want string
	}{
		"simple": {
			args{
				content: "test",
			},
			"4d967a30111bf29f0eba01c448b375c1629b2fed01cdfcc3aed91f1b57d5dd5e",
		},
	}

	for intention, tc := range cases {
		t.Run(intention, func(t *testing.T) {
			if got := New(tc.args.content); got != tc.want {
				t.Errorf("New() = `%s`, want `%s`", got, tc.want)
			}
		})
	}
}

func TestJSON(t *testing.T) {
	t.Parallel()

	type payload struct {
		ID uint64
	}

	type args struct {
		content payload
	}

	cases := map[string]struct {
		args args
		want string
	}{
		"simple": {
			args{
				content: payload{
					ID: 8000,
				},
			},
			"4a0946ec5089c35bfcf65b5e03dce607a5908fe4dda87dfa71744354b5805779",
		},
	}

	for intention, testCase := range cases {
		intention := intention
		testCase := testCase

		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			if got := JSON(testCase.args.content); got != testCase.want {
				t.Errorf("JSON() = `%s`, want `%s`", got, testCase.want)
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
