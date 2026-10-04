package main

import "testing"

func TestParseCellar(t *testing.T) {
	cases := []struct {
		in, want string
		wantErr  bool
	}{
		{in: "", want: ""},
		{in: "  ", want: ""},
		{in: "local", want: localCellar},
		{in: "https://cellar-1.internal:8081/", want: "https://cellar-1.internal:8081"},
		{in: "http://10.0.0.7:8081", want: "http://10.0.0.7:8081"},
		{in: "Local", wantErr: true},
		{in: "off", wantErr: true},
		{in: "ftp://cellar-1", wantErr: true},
		{in: "https://", wantErr: true},
		{in: "cellar-1:8081", wantErr: true},
		{in: "https://user:secret@cellar-1", wantErr: true},
	}
	for _, c := range cases {
		got, err := parseCellar(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("parseCellar(%q) = %q, want an error", c.in, got)
			}
			continue
		}
		if err != nil || got != c.want {
			t.Errorf("parseCellar(%q) = %q, %v; want %q", c.in, got, err, c.want)
		}
	}
}

func TestCellarID(t *testing.T) {
	cases := []struct {
		name, env, host, want string
		wantErr               bool
	}{
		{name: "host without dots", host: "cellar-1", want: "cellar-1"},
		{name: "host is lower cased", host: "Cellar-1", want: "cellar-1"},
		{name: "explicit id wins over an address", env: "staging", host: "127.0.0.1", want: "staging"},
		{name: "explicit id is lower cased", env: " Staging ", host: "127.0.0.1", want: "staging"},
		{name: "an IP address has dots", host: "127.0.0.1", wantErr: true},
		{name: "a dotted name has dots", host: "cellar-1.internal", wantErr: true},
		{name: "an explicit id with a dot", env: "cellar.1", host: "cellar-1", wantErr: true},
		{name: "an explicit id starting with a hyphen", env: "-staging", host: "cellar-1", wantErr: true},
		{name: "nothing", host: "", wantErr: true},
	}
	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			t.Setenv("CELLAR_ID", c.env)
			got, err := cellarID(c.host)
			if c.wantErr {
				if err == nil {
					t.Fatalf("cellarID(%q) = %q, want an error", c.host, got)
				}
				return
			}
			if err != nil || got != c.want {
				t.Fatalf("cellarID(%q) = %q, %v; want %q", c.host, got, err, c.want)
			}
		})
	}
}
