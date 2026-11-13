package admin

import (
	"bytes"
	"log/slog"
	"strconv"
	"strings"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/shop"
)

// A product request asks for the publishing right once and lets that one
// answer decide everything. Asked twice, a transient failure of the first
// lookup and a success of the second split the request: the second said "not
// locked", the first said "store a draft", and a publisher's save took a live
// product offline without a word. A failing lookup logs one ERROR line each
// time it is asked, so the lines count the questions.
func TestAProductRequestAsksForThePublishingRightOnce(t *testing.T) {
	const nobody = int64(424242)

	var logged bytes.Buffer
	previous := slog.Default()
	slog.SetDefault(slog.New(slog.NewTextHandler(&logged, nil)))
	t.Cleanup(func() { slog.SetDefault(previous) })

	h, sm, _, ws := newTestAdmin(t)
	products := withShop(h)
	live := seedShopProduct(t, h, products, ws.ID, "online", shop.StatusPublished)
	id := strconv.FormatInt(live.ID, 10)

	questions := func() int { return strings.Count(logged.String(), "rights lookup failed") }

	t.Run("saving", func(t *testing.T) {
		logged.Reset()
		serveAs(t, h, sm, nobody, h.HandleProductForm,
			postForm(productAddress(ws.ID, live.ID), productForm("Umbenannt", 0), productRoute(ws.ID, id)))
		if n := questions(); n != 1 {
			t.Errorf("the save asked for the publishing right %d times, want once", n)
		}
	})

	t.Run("the form", func(t *testing.T) {
		logged.Reset()
		getAs(t, sm, putUser(sm, nobody), h.HandleProductForm, productAddress(ws.ID, live.ID), productRoute(ws.ID, id))
		if n := questions(); n != 1 {
			t.Errorf("the form asked for the publishing right %d times, want once", n)
		}
	})

	t.Run("deleting", func(t *testing.T) {
		logged.Reset()
		serveAs(t, h, sm, nobody, h.HandleProductDelete,
			postForm(productAddress(ws.ID, live.ID)+"/delete", nil, productRoute(ws.ID, id)))
		if n := questions(); n != 1 {
			t.Errorf("the delete asked for the publishing right %d times, want once", n)
		}
	})
}
