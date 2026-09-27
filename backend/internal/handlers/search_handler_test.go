// backend/internal/handlers/search_handler_test.go
package handlers

import "testing"

func TestIlikePattern(t *testing.T) {
	cases := []struct {
		name string
		q    string
		want string
	}{
		{"plain query wrapped in wildcards", "acme", "%acme%"},
		{"percent sign escaped literally", "50%", "%50\\%%"},
		{"underscore escaped literally", "a_b", "%a\\_b%"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			got := ilikePattern(tc.q)
			if got != tc.want {
				t.Fatalf("got %q, want %q", got, tc.want)
			}
		})
	}
}
