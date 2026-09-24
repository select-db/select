package cellar

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		mode    Mode
		url     string
		wantErr bool
	}{
		{in: "", mode: Disabled},
		{in: "  ", mode: Disabled},
		{in: "local", mode: Local},
		{in: "https://cellar-1.internal:8081/", mode: Remote, url: "https://cellar-1.internal:8081"},
		{in: "http://10.0.0.7:8081", mode: Remote, url: "http://10.0.0.7:8081"},
		{in: "Local", wantErr: true},
		{in: "off", wantErr: true},
		{in: "ftp://cellar-1", wantErr: true},
		{in: "https://", wantErr: true},
		{in: "cellar-1:8081", wantErr: true},
	}
	for _, c := range cases {
		got, err := Parse(c.in)
		if c.wantErr {
			if err == nil {
				t.Errorf("Parse(%q) = %+v, want an error", c.in, got)
			}
			continue
		}
		if err != nil {
			t.Errorf("Parse(%q): %v", c.in, err)
			continue
		}
		if got.Mode != c.mode || got.URL != c.url {
			t.Errorf("Parse(%q) = %+v, want mode %v url %q", c.in, got, c.mode, c.url)
		}
	}
}
