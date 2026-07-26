package schema

import "testing"

func TestParseDefaultLiteral(t *testing.T) {
	cases := map[string]string{
		"'local'::text":            "local",
		"'deny'::text":             "deny",
		"'local'":                  "local",
		"'it''s'::text":            "it's",
		"gen_random_uuid()":        "",
		"now()":                    "",
		"nextval('seq'::regclass)": "",
		"":                         "",
	}
	for in, want := range cases {
		if got := parseDefaultLiteral(in); got != want {
			t.Errorf("parseDefaultLiteral(%q) = %q, want %q", in, got, want)
		}
	}
}
