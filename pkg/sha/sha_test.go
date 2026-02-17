package sha

import "testing"

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
			"b2a15a2509fce509931bc2204fd197def8adb7223ee697d746c71c7ce128b325",
		},
	}

	for intention, testCase := range cases {
		t.Run(intention, func(t *testing.T) {
			t.Parallel()

			if got := JSON(testCase.args.content); got != testCase.want {
				t.Errorf("JSON() = `%s`, want `%s`", got, testCase.want)
			}
		})
	}
}

func BenchmarkJSON(b *testing.B) {
	type testStruct struct {
		ID int
	}

	item := testStruct{}

	for i := 0; i < b.N; i++ {
		JSON(item)
	}
}
