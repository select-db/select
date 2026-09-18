package workspace

import (
	"bytes"
	"encoding/base64"
	"errors"
	"fmt"
	"image"
	"image/png"
	"strings"
)

const LogoSize = 128

// A 128x128 RGBA PNG cannot exceed ~66 KB even when every pixel is noise, so this
// cap rejects an attack but never an honest image. Kept in step with the CHECK
// constraint on workspace.logo in both databases.
const MaxLogoBase64Bytes = 96 * 1024

// Larger than the stored cap: the caller's PNG may compress worse than ours.
const maxLogoInputBase64Bytes = 128 * 1024

// Leaves room for the JSON envelope around the base64.
const MaxLogoRequestBytes = 160 * 1024

// Base64 of the PNG magic number, which every base64 PNG starts with.
const logoBase64Prefix = "iVBORw0KGgo"

var ErrInvalidLogo = errors.New("invalid logo")

func invalidLogo(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidLogo, reason)
}

// NormalizeLogo turns caller-supplied base64 into the base64 of a PNG produced by
// our own encoder. It is the only way a value reaches workspace.logo.
//
// Two things here are load-bearing. DecodeConfig runs before Decode, so an image
// declaring enormous dimensions is rejected from its header before a pixel buffer
// is allocated. And re-encoding drops every ancillary chunk (tEXt/iTXt/eXIf,
// colour profiles) and anything appended after IEND, so a polyglot file cannot
// survive the round trip.
func NormalizeLogo(input string) (string, error) {
	input = strings.TrimSpace(input)
	if input == "" {
		return "", invalidLogo("empty image")
	}
	if len(input) > maxLogoInputBase64Bytes {
		return "", invalidLogo("image too large")
	}

	raw, err := base64.StdEncoding.DecodeString(input)
	if err != nil {
		return "", invalidLogo("not valid base64")
	}

	cfg, format, err := image.DecodeConfig(bytes.NewReader(raw))
	if err != nil {
		return "", invalidLogo("not a readable image")
	}
	if format != "png" {
		return "", invalidLogo("image must be a PNG")
	}
	if cfg.Width != LogoSize || cfg.Height != LogoSize {
		return "", invalidLogo(fmt.Sprintf("image must be %dx%d pixels", LogoSize, LogoSize))
	}

	img, err := png.Decode(bytes.NewReader(raw))
	if err != nil {
		return "", invalidLogo("not a readable image")
	}
	// The bounds, not the header, are what we are about to encode.
	if b := img.Bounds(); b.Dx() != LogoSize || b.Dy() != LogoSize {
		return "", invalidLogo(fmt.Sprintf("image must be %dx%d pixels", LogoSize, LogoSize))
	}

	var out bytes.Buffer
	enc := png.Encoder{CompressionLevel: png.BestCompression}
	if err := enc.Encode(&out, img); err != nil {
		return "", invalidLogo("could not re-encode image")
	}

	encoded := base64.StdEncoding.EncodeToString(out.Bytes())
	if len(encoded) > MaxLogoBase64Bytes {
		return "", invalidLogo("image too large")
	}
	if !strings.HasPrefix(encoded, logoBase64Prefix) {
		return "", invalidLogo("could not re-encode image")
	}
	return encoded, nil
}
