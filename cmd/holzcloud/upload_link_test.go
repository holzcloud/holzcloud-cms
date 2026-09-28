package main

import (
	"bytes"
	"context"
	"encoding/json"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/ai"
	"github.com/holzcloud/holzcloud-cms/internal/domain"
	"github.com/holzcloud/holzcloud-cms/internal/media"
)

// An upload link, through the real route table: the tool under /ai hands out
// the address, the mount under ai.UploadPath receives the file, and the admin's
// own intake stores it. The three are wired in different places, and only a
// request that runs through all of them shows that they meet.
func TestAnUploadLinkGoesThroughTheRouter(t *testing.T) {
	ctx := context.Background()
	handler, _, database := testRouterWith(t, routerTweaks{mcp: true})

	ws, err := domain.NewStore(database).CreateWebsite(ctx, "Hof", "")
	if err != nil {
		t.Fatal(err)
	}
	key, _, err := ai.NewStore(database).Issue(ctx, "werkzeug", 0, true, 0)
	if err != nil {
		t.Fatal(err)
	}

	// The link, over /ai as an assistant asks for it.
	call, _ := json.Marshal(map[string]any{
		"jsonrpc": "2.0", "id": 1, "method": "tools/call",
		"params": map[string]any{"name": "create_upload_link", "arguments": map[string]any{
			"website": ws.ID, "file_name": "hof.png", "alt_text": "Der Hof",
		}},
	})
	req := httptest.NewRequest("POST", "/ai", bytes.NewReader(call))
	req.Host = "admin.test"
	req.Header.Set("Authorization", "Bearer "+key)
	req.Header.Set("Content-Type", "application/json")
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)
	if rec.Code != http.StatusOK {
		t.Fatalf("/ai: %d %s", rec.Code, rec.Body)
	}
	var rpc struct {
		Result struct {
			IsError bool `json:"isError"`
			Content []struct {
				Text string `json:"text"`
			} `json:"content"`
		} `json:"result"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &rpc); err != nil {
		t.Fatalf("/ai answer: %v: %s", err, rec.Body)
	}
	if rpc.Result.IsError || len(rpc.Result.Content) == 0 {
		t.Fatalf("create_upload_link refused: %s", rec.Body)
	}
	var link map[string]any
	if err := json.Unmarshal([]byte(rpc.Result.Content[0].Text), &link); err != nil {
		t.Fatalf("create_upload_link answer: %v", err)
	}
	uploadURL, _ := link["upload_url"].(string)
	if !strings.HasPrefix(uploadURL, "http://admin.test/ai/upload/") {
		t.Fatalf("upload_url = %q", uploadURL)
	}
	if link["method"] != "PUT" {
		t.Errorf("method = %v, want PUT", link["method"])
	}
	u, err := url.Parse(uploadURL)
	if err != nil {
		t.Fatal(err)
	}

	// A real picture, so the admin's checks have something to read.
	img := image.NewRGBA(image.Rect(0, 0, 64, 48))
	for y := 0; y < 48; y++ {
		for x := 0; x < 64; x++ {
			img.Set(x, y, color.RGBA{uint8(x * 4), uint8(y * 5), 120, 255})
		}
	}
	var pic bytes.Buffer
	if err := png.Encode(&pic, img); err != nil {
		t.Fatal(err)
	}

	send := func(method string) *httptest.ResponseRecorder {
		req := httptest.NewRequest(method, u.Path, bytes.NewReader(pic.Bytes()))
		req.Host = "admin.test"
		rec := httptest.NewRecorder()
		handler.ServeHTTP(rec, req)
		return rec
	}

	rec = send("PUT")
	if rec.Code != http.StatusCreated {
		t.Fatalf("PUT: %d %s", rec.Code, rec.Body)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("Content-Type = %q", ct)
	}
	var stored map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &stored); err != nil {
		t.Fatalf("PUT answer: %v: %s", err, rec.Body)
	}
	if stored["description"] != "Der Hof" {
		t.Errorf("description = %v", stored["description"])
	}
	if stored["mime"] != "image/png" {
		t.Errorf("mime = %v", stored["mime"])
	}
	idNum, _ := stored["id"].(float64)
	if idNum <= 0 {
		t.Fatalf("id = %v", stored["id"])
	}
	m, err := media.NewStore(database).GetByID(ctx, int64(idNum))
	if err != nil || m == nil {
		t.Fatalf("media %s: %v", strconv.FormatFloat(idNum, 'f', 0, 64), err)
	}
	if m.AltText != "Der Hof" || m.WebsiteID != ws.ID {
		t.Errorf("stored alt %q website %d, want %q and %d", m.AltText, m.WebsiteID, "Der Hof", ws.ID)
	}

	// Used once, the link is gone — and the answer is the handler's, not the
	// public site's page.
	rec = send("PUT")
	if rec.Code != http.StatusNotFound {
		t.Errorf("second PUT: %d, want 404", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("second PUT Content-Type = %q, want the handler's JSON", ct)
	}

	rec = send("GET")
	if rec.Code != http.StatusMethodNotAllowed {
		t.Errorf("GET: %d, want 405", rec.Code)
	}
	if allow := rec.Header().Get("Allow"); allow != "PUT, POST" {
		t.Errorf("Allow = %q", allow)
	}
}
