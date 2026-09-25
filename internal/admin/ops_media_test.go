package admin

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/album"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// The media tools of the AI connection against the real handler: the upload
// has to go through the very intake the upload screen uses, and the crop
// through the crop screen's function. internal/ai tests the tools with a
// stand-in; only here is the Op side real.

type aiMediaFixture struct {
	h   *Handler
	ts  *httptest.Server
	key string
	ws  int64
}

func setUpAIMedia(t *testing.T) aiMediaFixture {
	t.Helper()
	h, _, database, ws := mediaAdmin(t)
	h.SetAlbumStore(album.NewStore(database))

	tokens := ai.NewStore(database)
	key, _, err := tokens.Issue(context.Background(), "werkzeug", 0, true, 0)
	if err != nil {
		t.Fatal(err)
	}
	srv := ai.NewServer(tokens, "Test", slog.New(slog.DiscardHandler), ai.Tools(ai.Deps{
		Domains: h.domains, Pages: h.pages, Media: h.mediaStore,
		Ops: h,
		Limits: ai.Limits{
			DataDir: h.cfg.DataDir, MaxMediaSize: h.cfg.MaxMediaSize,
			MaxVideoSize: h.cfg.MaxVideoSize, MaxMegapixels: h.cfg.MaxMegapixels,
		},
	}))
	srv.SetMaxRequestBytes(h.cfg.MaxMediaSize)
	ts := httptest.NewServer(srv)
	t.Cleanup(ts.Close)
	return aiMediaFixture{h: h, ts: ts, key: key, ws: ws.ID}
}

// tool calls one tool and says whether it failed.
func (f aiMediaFixture) tool(t *testing.T, name string, args map[string]any) (map[string]any, string) {
	t.Helper()
	raw, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": name, "arguments": args},
	})
	req, _ := http.NewRequest(http.MethodPost, f.ts.URL, bytes.NewReader(raw))
	req.Header.Set("Authorization", "Bearer "+f.key)
	res, err := f.ts.Client().Do(req)
	if err != nil {
		t.Fatal(err)
	}
	defer res.Body.Close()
	var body struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
		Error any `json:"error"`
	}
	if err := json.NewDecoder(res.Body).Decode(&body); err != nil || body.Error != nil || len(body.Result.Content) == 0 {
		t.Fatalf("%s: %v %v", name, err, body.Error)
	}
	text := body.Result.Content[0].Text
	if body.Result.IsError {
		return nil, text
	}
	var out map[string]any
	if err := json.Unmarshal([]byte(text), &out); err != nil {
		t.Fatalf("%s: %q", name, text)
	}
	return out, ""
}

// photo is a JPEG with an EXIF block carrying a place, as a phone writes it.
func photo(t *testing.T, w, h int) []byte {
	t.Helper()
	img := image.NewRGBA(image.Rect(0, 0, w, h))
	for y := 0; y < h; y++ {
		for x := 0; x < w; x++ {
			img.Set(x, y, color.RGBA{uint8(x), uint8(y), 90, 255})
		}
	}
	var buf bytes.Buffer
	if err := jpeg.Encode(&buf, img, &jpeg.Options{Quality: 90}); err != nil {
		t.Fatal(err)
	}
	payload := []byte("Exif\x00\x00GPS-Bauernhof-Musterweg-7")
	app1 := []byte{0xFF, 0xE1, byte((len(payload) + 2) >> 8), byte(len(payload) + 2)}
	jpg := buf.Bytes()
	out := append([]byte{}, jpg[:2]...)
	out = append(out, app1...)
	out = append(out, payload...)
	return append(out, jpg[2:]...)
}

func TestTheAIUploadIsTheScreensUpload(t *testing.T) {
	f := setUpAIMedia(t)
	content := photo(t, 800, 600)

	out, failed := f.tool(t, "upload_media", map[string]any{
		"website": f.ws, "file_name": "../../etc/hof.jpg", "alt_text": "Der Hof",
		"content_base64": base64.StdEncoding.EncodeToString(content),
	})
	if failed != "" {
		t.Fatalf("upload_media: %s", failed)
	}
	if out["file"] != "hof.jpg" || out["mime"] != "image/jpeg" || out["description"] != "Der Hof" {
		t.Errorf("upload_media = %v", out)
	}
	if out["width"] != float64(800) {
		t.Errorf("the dimensions of the variants are not recorded: %v", out)
	}
	id := int64(out["id"].(float64))
	m, _ := f.h.mediaStore.GetByID(context.Background(), id)
	stored, err := os.ReadFile(media.Path(f.h.cfg.DataDir, f.ws, m.Filename))
	if err != nil {
		t.Fatal(err)
	}
	if bytes.Contains(stored, []byte("Bauernhof-Musterweg")) {
		t.Error("the EXIF block with the place was stored")
	}
	if v, _ := f.h.mediaStore.VariantsFor(context.Background(), id); len(v) == 0 {
		t.Error("no scaled copies were made")
	}

	// The same bytes again: nothing stored, the existing file named.
	again, failed := f.tool(t, "upload_media", map[string]any{
		"website": f.ws, "file_name": "nochmal.jpg",
		"content_base64": base64.StdEncoding.EncodeToString(content),
	})
	if failed != "" || again["id"] != out["id"] || !strings.Contains(again["note"].(string), "already exists") {
		t.Errorf("duplicate: %v %s", again, failed)
	}

	// The checks of media.Check: bytes, not the name; and an SVG that loads
	// from elsewhere.
	if _, failed := f.tool(t, "upload_media", map[string]any{
		"website": f.ws, "file_name": "bild.png",
		"content_base64": base64.StdEncoding.EncodeToString([]byte("\x7fELF not a picture at all")),
	}); !strings.Contains(failed, "not allowed") {
		t.Errorf("a program called .png: %q", failed)
	}
	svg := `<svg xmlns="http://www.w3.org/2000/svg"><image href="https://example.com/x.png"/></svg>`
	if _, failed := f.tool(t, "upload_media", map[string]any{
		"website": f.ws, "file_name": "logo.svg",
		"content_base64": base64.StdEncoding.EncodeToString([]byte(svg)),
	}); !strings.Contains(failed, "example.com") {
		t.Errorf("an SVG loading from elsewhere: %q", failed)
	}
	// Too large for the installation's limit.
	f.h.cfg.MaxMediaSize = 1 << 10
	if _, failed := f.tool(t, "upload_media", map[string]any{
		"website": f.ws, "file_name": "gross.jpg",
		"content_base64": base64.StdEncoding.EncodeToString(photo(t, 300, 300)),
	}); !strings.Contains(failed, "larger than allowed") {
		t.Errorf("too large: %q", failed)
	}
}

func TestTheAICropIsTheScreensCrop(t *testing.T) {
	f := setUpAIMedia(t)
	m := seedImage(t, f.h, f.ws, "weide.jpg", 400, 200)

	out, failed := f.tool(t, "crop_media", map[string]any{"id": m.ID, "ratio": "1-1", "rotation": 0})
	if failed != "" {
		t.Fatalf("crop_media: %s", failed)
	}
	if out["width"] != float64(200) || out["height"] != float64(200) {
		t.Errorf("crop to a square: %v", out)
	}
	if !media.HasOriginal(media.WebsiteDir(f.h.cfg.DataDir, f.ws), m.Filename) {
		t.Error("the upload was not kept beside the crop")
	}

	out, failed = f.tool(t, "crop_media", map[string]any{"id": m.ID, "rotation": 90})
	if failed != "" || out["width"] != float64(200) || out["height"] != float64(400) {
		t.Errorf("a quarter turn: %v %s", out, failed)
	}
	if _, failed := f.tool(t, "crop_media", map[string]any{"id": m.ID, "rotation": 45}); failed == "" {
		t.Error("a rotation of 45 degrees was accepted")
	}

	out, failed = f.tool(t, "restore_media_original", map[string]any{"id": m.ID})
	if failed != "" || out["width"] != float64(400) || out["height"] != float64(200) {
		t.Errorf("restore: %v %s", out, failed)
	}
}

func TestTheAIAlbumUsesTheScreensPictureCheck(t *testing.T) {
	f := setUpAIMedia(t)
	ctx := context.Background()
	picture := seedImage(t, f.h, f.ws, "a.jpg", 40, 40)
	doc, _ := f.h.mediaStore.Create(ctx, f.ws, "p.pdf", "preise.pdf", "application/pdf", 10, "pdf")

	al, failed := f.tool(t, "create_album", map[string]any{"website": f.ws, "name": "Weide"})
	if failed != "" {
		t.Fatal(failed)
	}
	out, failed := f.tool(t, "add_album_images", map[string]any{
		"website": f.ws, "album": al["id"],
		"images": []any{map[string]any{"media": doc.ID}, map[string]any{"media": picture.ID}},
	})
	if failed != "" || out["added"] != float64(1) {
		t.Fatalf("add_album_images: %v %s", out, failed)
	}
	reason := out["refused"].([]any)[0].(map[string]any)["reason"].(string)
	if !strings.Contains(reason, "does not belong") {
		t.Errorf("a PDF in an album: %q", reason)
	}
}
