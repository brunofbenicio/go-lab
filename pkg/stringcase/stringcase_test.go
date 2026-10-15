package stringcase

import (
	"slices"
	"testing"
)

func TestWords(t *testing.T) {
	tests := []struct {
		in   string
		want []string
	}{
		{"helloWorld", []string{"hello", "World"}},
		{"HelloWorld", []string{"Hello", "World"}},
		{"hello_world", []string{"hello", "world"}},
		{"hello-world", []string{"hello", "world"}},
		{"  multiple   spaces  ", []string{"multiple", "spaces"}},
		{"HTTPServer", []string{"HTTP", "Server"}},
		{"userID", []string{"user", "ID"}},
		{"parseHTTPResponse", []string{"parse", "HTTP", "Response"}},
		{"user2Name", []string{"user2", "Name"}},
		{"HTTP2Server", []string{"HTTP2", "Server"}},
		{"ÁguaPotável", []string{"Água", "Potável"}},
		{"already_Mixed-Case words", []string{"already", "Mixed", "Case", "words"}},
		{"", nil},
		{"___", nil},
	}
	for _, tt := range tests {
		if got := Words(tt.in); !slices.Equal(got, tt.want) {
			t.Errorf("Words(%q) = %q, want %q", tt.in, got, tt.want)
		}
	}
}

func TestConversions(t *testing.T) {
	tests := []struct {
		in                                  string
		camel, pascal, snake, kebab, scream string
	}{
		{"hello world", "helloWorld", "HelloWorld", "hello_world", "hello-world", "HELLO_WORLD"},
		{"HelloWorld", "helloWorld", "HelloWorld", "hello_world", "hello-world", "HELLO_WORLD"},
		{"HTTPServer", "httpServer", "HttpServer", "http_server", "http-server", "HTTP_SERVER"},
		{"user_id", "userId", "UserId", "user_id", "user-id", "USER_ID"},
		{"get2FAToken", "get2FaToken", "Get2FaToken", "get2_fa_token", "get2-fa-token", "GET2_FA_TOKEN"},
		{"água potável", "águaPotável", "ÁguaPotável", "água_potável", "água-potável", "ÁGUA_POTÁVEL"},
		{"x", "x", "X", "x", "x", "X"},
		{"", "", "", "", "", ""},
	}
	for _, tt := range tests {
		t.Run(tt.in, func(t *testing.T) {
			for name, c := range map[string]struct{ got, want string }{
				"Camel":          {Camel(tt.in), tt.camel},
				"Pascal":         {Pascal(tt.in), tt.pascal},
				"Snake":          {Snake(tt.in), tt.snake},
				"Kebab":          {Kebab(tt.in), tt.kebab},
				"ScreamingSnake": {ScreamingSnake(tt.in), tt.scream},
			} {
				if c.got != c.want {
					t.Errorf("%s(%q) = %q, want %q", name, tt.in, c.got, c.want)
				}
			}
		})
	}
}

func TestRoundTrip(t *testing.T) {
	for _, in := range []string{"user_account_id", "payment-method"} {
		if got := Snake(Camel(in)); got != Snake(in) {
			t.Errorf("Snake(Camel(%q)) = %q, want %q", in, got, Snake(in))
		}
		if got := Kebab(Pascal(in)); got != Kebab(in) {
			t.Errorf("Kebab(Pascal(%q)) = %q, want %q", in, got, Kebab(in))
		}
	}
}
