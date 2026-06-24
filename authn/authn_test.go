package authn

import "testing"

func TestExtractBearer(t *testing.T) {
	cases := []struct {
		in, want string
		ok       bool
	}{
		{"Bearer abc", "abc", true},
		{"bearer abc", "abc", true},
		{" Bearer   abc ", "abc", true},
		{"Basic abc", "", false},
		{"", "", false},
	}
	for _, tc := range cases {
		got, ok := ExtractBearer(tc.in)
		if got != tc.want || ok != tc.ok {
			t.Fatalf("ExtractBearer(%q)=%q,%v want %q,%v", tc.in, got, ok, tc.want, tc.ok)
		}
	}
}
