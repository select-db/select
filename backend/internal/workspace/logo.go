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

// LogoSize is the exact edge length, in pixels, of a stored workspace logo. The
// client normalizes to it on a canvas before upload; the server enforces it
// rather than resizing, so the stored bytes are always the same shape.
const LogoSize = 128

// MaxLogoBase64Bytes bounds the stored base64. A 128x128 RGBA PNG cannot exceed
// ~66 KB even when every pixel is noise (128*128*4 raw plus filter and zlib
// overhead), so 96 KB is above anything our own encoder can produce for a valid
// logo: the cap can reject an attack but never an honest image. Kept in step
// with the CHECK constraint on workspace.logo in both databases.
const MaxLogoBase64Bytes = 96 * 1024

// maxLogoInputBase64Bytes bounds what a caller may send before decoding. Larger
// than the stored cap because the caller's PNG is re-encoded (and may compress
// worse than ours), but still small enough that no request can allocate much.
const maxLogoInputBase64Bytes = 128 * 1024

// MaxLogoRequestBytes bounds the HTTP body, leaving room for the JSON envelope
// around the base64.
const MaxLogoRequestBytes = 160 * 1024

// logoBase64Prefix is the base64 encoding of the PNG magic number
// (89 50 4E 47 0D 0A 1A 0A), which every base64 PNG starts with. Asserted on the
// way out as a cheap check that we really did emit a PNG, and enforced by the
// CHECK constraint on the column.
const logoBase64Prefix = "iVBORw0KGgo"

// ErrInvalidLogo is returned for every rejection reason. The message is safe to
// hand back to the caller: it says what is wrong with the image, never anything
// about the decoder's internals.
var ErrInvalidLogo = errors.New("invalid logo")

func invalidLogo(reason string) error {
	return fmt.Errorf("%w: %s", ErrInvalidLogo, reason)
}

// NormalizeLogo turns caller-supplied base64 into the base64 of a PNG produced
// by our own encoder. It is the only way a value reaches workspace.logo.
//
// The order matters. DecodeConfig runs before Decode so an image declaring
// enormous dimensions is rejected from its header, before any pixel buffer is
// allocated. Decoding is restricted to PNG (the wire format the client always
// sends, because it re-encodes on a canvas first), so no other image parser is
// ever reachable from this input, and SVG — the one image format that can carry
// script — has no decoder here at all.
//
// Re-encoding is what makes the output trustworthy: it drops every ancillary
// chunk (tEXt/iTXt/eXIf metadata, colour profiles) and any bytes appended after
// IEND, so a polyglot file cannot survive the round trip.
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
	// Belt and braces: DecodeConfig and Decode read the same header, but the
	// bounds are what we are actually about to encode.
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
