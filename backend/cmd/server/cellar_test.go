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
