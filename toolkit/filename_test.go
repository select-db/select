package toolkit

import "testing"

func TestSafeFileName(t *testing.T) {
	for name, want := range map[string]string{
		"notes":               "notes",
		`a/b\c:d*e?f"g<h>i|j`: "a_b_c_d_e_f_g_h_i_j",
		"tab\there":           "tab_here",
		"new\nline":           "new_line",
		"  padded  ":          "padded",
		"   ":                 "fallback",
		"":                    "fallback",
		"café.v2":             "café.v2",
	} {
		if got := SafeFileName(name, "fallback"); got != want {
			t.Errorf("SafeFileName(%q) = %q, want %q", name, got, want)
		}
	}
}
