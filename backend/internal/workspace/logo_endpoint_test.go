package workspace_test

import (
	"bytes"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"

	"backend/e2e"
	"backend/internal/workspace"
)

// squarePNGBase64 builds a valid logo-shaped PNG for the happy paths.
func squarePNGBase64(t *testing.T, size int) string {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, size, size))
	for y := range size {
		for x := range size {
			img.Set(x, y, color.RGBA{R: uint8(x), G: uint8(y), B: 0x40, A: 0xff})
		}
	}
	var buf bytes.Buffer
	require.NoError(t, png.Encode(&buf, img))
	return base64.StdEncoding.EncodeToString(buf.Bytes())
}

func storedLogo(t *testing.T, f e2e.Fixture) (string, bool) {
	t.Helper()
	var logo *string
	require.NoError(t, f.Conn.QueryRow(`SELECT logo FROM app.workspace WHERE id = $1`, f.Actor.WorkspaceID).Scan(&logo))
	if logo == nil {
		return "", false
	}
	return *logo, true
}

func TestLogo_UploadStoresReEncodedPNG(t *testing.T) {
	f := e2e.Setup(t)

	rec := e2e.Do(t, f.H, http.MethodPut, "/workspaces/"+f.Actor.WorkspaceID+"/logo", f.Actor.Token,
		map[string]any{"logo": squarePNGBase64(t, workspace.LogoSize)})
	require.Equalf(t, http.StatusOK, rec.Code, "upload: %s", rec.Body.String())

	var body struct {
		Logo *string `json:"logo"`
	}
	require.NoError(t, json.Unmarshal(rec.Body.Bytes(), &body))
	require.NotNil(t, body.Logo)

	stored, ok := storedLogo(t, f)
	require.True(t, ok, "logo should be stored")
	require.Equal(t, *body.Logo, stored)
	require.True(t, len(stored) <= workspace.MaxLogoBase64Bytes)

	raw, err := base64.StdEncoding.DecodeString(stored)
	require.NoError(t, err)
	cfg, err := png.DecodeConfig(bytes.NewReader(raw))
	require.NoError(t, err)
	require.Equal(t, workspace.LogoSize, cfg.Width)
	require.Equal(t, workspace.LogoSize, cfg.Height)
}

func TestLogo_UploadBumpsUpdatedAtSoMembersPullIt(t *testing.T) {
	f := e2e.Setup(t)

	var before string
	require.NoError(t, f.Conn.QueryRow(`SELECT updated_at FROM app.workspace WHERE id = $1`, f.Actor.WorkspaceID).Scan(&before))

	rec := e2e.Do(t, f.H, http.MethodPut, "/workspaces/"+f.Actor.WorkspaceID+"/logo", f.Actor.Token,
		map[string]any{"logo": squarePNGBase64(t, workspace.LogoSize)})
	require.Equalf(t, http.StatusOK, rec.Code, "upload: %s", rec.Body.String())

	var after string
	require.NoError(t, f.Conn.QueryRow(`SELECT updated_at FROM app.workspace WHERE id = $1`, f.Actor.WorkspaceID).Scan(&after))
	require.NotEqual(t, before, after, "updated_at must move or members never pull the logo")
}

func TestLogo_RejectsBadImages(t *testing.T) {
	cases := []struct {
		name string
		logo any
	}{
		{"not an image", base64.StdEncoding.EncodeToString([]byte("this is not an image at all"))},
		{"SVG", base64.StdEncoding.EncodeToString([]byte(`<svg xmlns="http://www.w3.org/2000/svg"><script>alert(1)</script></svg>`))},
		{"wrong size", squarePNGBase64(t, 64)},
		{"data URL", "data:image/png;base64," + squarePNGBase64(t, workspace.LogoSize)},
		{"empty", ""},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			f := e2e.Setup(t)
			rec := e2e.Do(t, f.H, http.MethodPut, "/workspaces/"+f.Actor.WorkspaceID+"/logo", f.Actor.Token,
				map[string]any{"logo": tc.logo})
			require.Equalf(t, http.StatusBadRequest, rec.Code, "want 400: %s", rec.Body.String())

			_, ok := storedLogo(t, f)
			require.False(t, ok, "nothing should be stored for a rejected image")
		})
	}
}

func TestLogo_DeniedWithoutSettingsWrite(t *testing.T) {
	f := e2e.Setup(t)
	token := nonOwnerToken(t, f)

	rec := e2e.Do(t, f.H, http.MethodPut, "/workspaces/"+f.Actor.WorkspaceID+"/logo", token,
		map[string]any{"logo": squarePNGBase64(t, workspace.LogoSize), "workspace_id": f.Actor.WorkspaceID})
	require.Equalf(t, http.StatusForbidden, rec.Code, "want 403: %s", rec.Body.String())

	_, ok := storedLogo(t, f)
	require.False(t, ok, "a denied caller must not store a logo")
}

// The column constraint is the last line of defence: even if a handler bug let
// something through, the database must refuse anything that is not base64 PNG.
func TestLogo_ColumnConstraintRejectsNonPNG(t *testing.T) {
	f := e2e.Setup(t)

	for _, value := range []string{
		"PHN2ZyB4bWxucz0iaHR0cDovL3d3dy53My5vcmcvMjAwMC9zdmciPjwvc3ZnPg==", // base64 SVG
		"data:image/png;base64,iVBORw0KGgoAAAA",                            // a value carrying its own media type
		"iVBORw0KGgo<script>alert(1)</script>",                             // PNG prefix, non-base64 tail
	} {
		_, err := f.Conn.Exec(`UPDATE app.workspace SET logo = $2 WHERE id = $1`, f.Actor.WorkspaceID, value)
		require.Errorf(t, err, "database accepted %q", value)
	}

	// Oversized, even with a valid PNG prefix.
	oversized := "iVBORw0KGgo" + bytes.NewBuffer(bytes.Repeat([]byte("A"), workspace.MaxLogoBase64Bytes+16)).String()
	_, err := f.Conn.Exec(`UPDATE app.workspace SET logo = $2 WHERE id = $1`, f.Actor.WorkspaceID, oversized)
	require.Error(t, err, "database accepted an oversized logo")
}

// The body cap has to sit above the membership middleware, which buffers the
// body itself when the request carries no X-Workspace-Id header.
func TestLogo_RejectsOversizedBody(t *testing.T) {
	f := e2e.Setup(t)

	huge := strings.Repeat("A", workspace.MaxLogoRequestBytes+1024)
	rec := e2e.Do(t, f.H, http.MethodPut, "/workspaces/"+f.Actor.WorkspaceID+"/logo", f.Actor.Token,
		map[string]any{"logo": huge})
	require.NotEqualf(t, http.StatusOK, rec.Code, "oversized body was accepted: %s", rec.Body.String())

	_, ok := storedLogo(t, f)
	require.False(t, ok, "nothing should be stored for an oversized body")
}

// The invariant the whole design rests on: logo is absent from the sync path's
// column list, so a commit cannot set it — or clear one already stored.
func TestLogo_SyncCommitCannotWriteLogo(t *testing.T) {
	f := e2e.Setup(t)

	rec := e2e.Do(t, f.H, http.MethodPut, "/workspaces/"+f.Actor.WorkspaceID+"/logo", f.Actor.Token,
		map[string]any{"logo": squarePNGBase64(t, workspace.LogoSize)})
	require.Equalf(t, http.StatusOK, rec.Code, "upload: %s", rec.Body.String())
	stored, ok := storedLogo(t, f)
	require.True(t, ok)

	// A commit that renames the workspace and tries to carry a logo alongside.
	e2e.SyncCommit(t, f.H, f.Actor, "update", "workspace", f.Actor.WorkspaceID, map[string]any{
		"id":   f.Actor.WorkspaceID,
		"name": "Renamed by sync",
		"logo": "iVBORw0KGgoAAAANSUhEUgAAAAEAAAABCAYAAAAfFcSJAAAADUlEQVR42mP8z8BQDwAEhQGAhKmMIQAAAABJRU5ErkJggg==",
	})

	var name string
	require.NoError(t, f.Conn.QueryRow(`SELECT name FROM app.workspace WHERE id = $1`, f.Actor.WorkspaceID).Scan(&name))
	require.Equal(t, "Renamed by sync", name, "the commit itself should have applied")

	after, ok := storedLogo(t, f)
	require.True(t, ok, "the upsert must not clear the logo")
	require.Equal(t, stored, after, "sync must not be able to write the logo")
}
