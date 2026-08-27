package workspace

import (
	"bytes"
	"encoding/base64"
	"image"
	"image/color"
	"image/png"
	"strings"
	"testing"
)

// pngOf builds a valid PNG of the given size, so a test can vary exactly one
// property of the input at a time.
func pngOf(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := range h {
		for x := range w {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x80, A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode fixture: %v", err)
	}
	return buf.Bytes()
}

func TestNormalizeLogo_AcceptsA128SquarePNG(t *testing.T) {
	in := base64.StdEncoding.EncodeToString(pngOf(t, LogoSize, LogoSize))

	out, err := NormalizeLogo(in)
	if err != nil {
		t.Fatalf("NormalizeLogo: %v", err)
	}
	if !strings.HasPrefix(out, logoBase64Prefix) {
		t.Fatalf("output is not a base64 PNG: %.16q", out)
	}
	if len(out) > MaxLogoBase64Bytes {
		t.Fatalf("output %d bytes exceeds the stored cap %d", len(out), MaxLogoBase64Bytes)
	}

	// The output must be our encoder's, decodable and still the right shape.
	raw, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("output is not valid base64: %v", err)
	}
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		t.Fatalf("output is not a readable PNG: %v", err)
	}
	if cfg.Width != LogoSize || cfg.Height != LogoSize {
		t.Fatalf("output is %dx%d, want %dx%d", cfg.Width, cfg.Height, LogoSize, LogoSize)
	}
}

// A worst-case logo — every pixel different, so the PNG barely compresses — must
// still fit the cap, or the endpoint would reject images honest users upload.
func TestNormalizeLogo_NoisyImageFitsTheCap(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, LogoSize, LogoSize))
	seed := uint32(1)
	for y := range LogoSize {
		for x := range LogoSize {
			seed = seed*1664525 + 1013904223 // deterministic, no test flake
			img.Set(x, y, color.RGBA{R: uint8(seed >> 24), G: uint8(seed >> 16), B: uint8(seed >> 8), A: 0xff})
		}
	}
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}

	out, err := NormalizeLogo(base64.StdEncoding.EncodeToString(buf.Bytes()))
	if err != nil {
		t.Fatalf("NormalizeLogo rejected a valid noisy logo: %v", err)
	}
	if len(out) > MaxLogoBase64Bytes {
		t.Fatalf("noisy logo encodes to %d bytes, over the %d cap", len(out), MaxLogoBase64Bytes)
	}
}

// Re-encoding must drop ancillary chunks: metadata and anything appended after
// IEND are the payload half of a polyglot file.
func TestNormalizeLogo_StripsMetadataAndTrailingBytes(t *testing.T) {
	img := image.NewRGBA(image.Rect(0, 0, LogoSize, LogoSize))
	var buf bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&buf, img); err != nil {
		t.Fatalf("encode: %v", err)
	}
	marker := []byte("<script>alert(1)</script>")
	tainted := append(buf.Bytes(), marker...)

	out, err := NormalizeLogo(base64.StdEncoding.EncodeToString(tainted))
	if err != nil {
		t.Fatalf("NormalizeLogo: %v", err)
	}
	raw, err := base64.StdEncoding.DecodeString(out)
	if err != nil {
		t.Fatalf("decode output: %v", err)
	}
	if bytes.Contains(raw, marker) {
		t.Fatal("trailing bytes survived the re-encode")
	}
}

func TestNormalizeLogo_Rejects(t *testing.T) {
	// A header claiming an enormous image: DecodeConfig must reject it on
	// dimensions before Decode ever allocates a pixel buffer for it.
	bomb := pngOf(t, 1, 1)
	// 0x00010000 x 0x00010000 written into IHDR's width/height fields.
	copy(bomb[16:24], []byte{0x00, 0x01, 0x00, 0x00, 0x00, 0x01, 0x00, 0x00})

	jpegLike := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10, 'J', 'F', 'I', 'F', 0x00}

	cases := []struct {
		name  string
		input string
	}{
		{"empty", ""},
		{"whitespace only", "   \n "},
		{"not base64", "!!!! not base64 !!!!"},
		{"data URL prefix", "data:image/png;base64," + base64.StdEncoding.EncodeToString(pngOf(t, LogoSize, LogoSize))},
		{"not an image", base64.StdEncoding.EncodeToString([]byte("just some bytes, long enough to sniff"))},
		{"SVG", base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg" width="128" height="128"><script>alert(1)</script></svg>`))},
		{"JPEG", base64.StdEncoding.EncodeToString(jpegLike)},
		{"wrong size", base64.StdEncoding.EncodeToString(pngOf(t, 64, 64))},
		{"non square", base64.StdEncoding.EncodeToString(pngOf(t, LogoSize, 64))},
		{"dimension bomb", base64.StdEncoding.EncodeToString(bomb)},
		{"oversized input", strings.Repeat("A", maxLogoInputBase64Bytes+4)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if _, err := NormalizeLogo(tc.input); err == nil {
				t.Fatalf("NormalizeLogo accepted %s", tc.name)
			}
		})
	}
}
