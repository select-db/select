package cellar

import "testing"

func TestParse(t *testing.T) {
	cases := []struct {
		in      string
		want    Config
		wantErr bool
	}{
		{in: "", want: Config{}},
		{in: "  ", want: Config{}},
		{in: "local", want: Config{Local: true}},
		{in: "https://cellar-1.internal:8081/", want: Config{URL: "https://cellar-1.internal:8081"}},
		{in: "http://10.0.0.7:8081", want: Config{URL: "http://10.0.0.7:8081"}},
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
		if err != nil || got != c.want {
			t.Errorf("Parse(%q) = %+v, %v; want %+v", c.in, got, err, c.want)
		}
	}
}
