package admin

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The shop's front door: four numbers, what is to be done, and how the weeks
// went. The numbers come from shop.OverviewOf, which the assistant's
// shop_overview tool reads as well; this file only words them for the screen.

type shopTask struct {
	// Kind is the label of the pill: payment, dispatch or stock.
	Kind string
	// Warn marks what is overdue rather than merely next.
	Warn bool
	Text string
	URL  string
	// Action is what the button says.
	Action string
}

type shopWeek struct {
	Label  string
	Amount string
	// Percent is the bar's height against the best week, 0–100.
	Percent int
	Last    bool
}

type shopOverviewData struct {
	web.LayoutData
	Revenue        string
	RevenueChange  int
	HasPrevious    bool
	OpenOrders     int
	AwaitPayment   int
	LowStock       int
	ProductsOnline int
	ProductDrafts  int
	Tasks          []shopTask
	Weeks          []shopWeek
	BestWeek       string
	HasOrders      bool
}

// HandleShopOverview shows the overview of one website's shop.
func (h *Handler) HandleShopOverview(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	cur := money.CurrencyFor(ws.Currency)
	ov, err := h.shopOverview(r.Context(), ws.ID, time.Now())
	if err != nil {
		return err
	}

	data := shopOverviewData{
		LayoutData:     web.NewLayoutData(r, h.sm, web.Titlef(r, "Shop – %s", ws.Name)),
		HasOrders:      ov.HasOrders,
		Revenue:        cur.Format(ov.ThisMonth),
		RevenueChange:  ov.RevenueChange,
		HasPrevious:    ov.HasPrevious,
		OpenOrders:     ov.OpenOrders,
		AwaitPayment:   ov.AwaitPayment,
		LowStock:       ov.LowStock,
		ProductsOnline: ov.ProductsOnline,
		ProductDrafts:  ov.ProductDrafts,
	}
	data.ActiveNav = "shop"
	data.CurrentWebsite = ws

	base := "/admin/websites/" + strconv.FormatInt(ws.ID, 10)
	for _, t := range ov.Tasks {
		switch t.Kind {
		case shop.TaskPayment:
			data.Tasks = append(data.Tasks, shopTask{
				Kind: web.T(r, "Payment"), Warn: t.Warn,
				Text:   web.Titlef(r, "Order %s has been waiting for its payment for %d days", t.OrderNumber, t.DaysWaiting),
				URL:    base + "/bestellungen/" + t.OrderNumber,
				Action: web.T(r, "Open"),
			})
		case shop.TaskDispatch:
			data.Tasks = append(data.Tasks, shopTask{
				Kind: web.T(r, "Dispatch"), Warn: t.Warn,
				Text:   web.Titlef(r, "Order %s is paid and waiting to be sent", t.OrderNumber),
				URL:    base + "/bestellungen/" + t.OrderNumber,
				Action: web.T(r, "Delivery note"),
			})
		case shop.TaskStock:
			data.Tasks = append(data.Tasks, shopTask{
				Kind: web.T(r, "Stock"), Warn: t.Warn,
				Text:   web.Titlef(r, "%s: %d left", t.ProductTitle, t.Stock),
				URL:    base + "/produkte/" + strconv.FormatInt(t.ProductID, 10),
				Action: web.T(r, "Restock"),
			})
		}
	}

	for _, wk := range ov.Weeks {
		data.Weeks = append(data.Weeks, shopWeek{
			Label: web.Titlef(r, "Wk %d", wk.Week), Amount: cur.Format(wk.Amount),
			Percent: wk.Percent, Last: wk.Current,
		})
	}
	if ov.Best > 0 {
		data.BestWeek = web.Titlef(r, "Best week %s: %s", web.Titlef(r, "Wk %d", ov.BestWeek), cur.Format(ov.Best))
	}

	return web.RenderAdmin(w, h.templates, r, "shop_overview", data)
}

// shopOverview reads what the overview is computed from — the screen and
// OpShopOverview both come through here.
func (h *Handler) shopOverview(ctx context.Context, websiteID int64, now time.Time) (shop.Overview, error) {
	var orders []*shop.Order
	if h.orders != nil {
		// A year of a workshop's orders; the chart needs eight weeks and the
		// month two, so this is plenty and still one query.
		var err error
		if orders, err = h.orders.List(ctx, websiteID, 1000); err != nil {
			return shop.Overview{}, err
		}
	}
	products, err := h.products.List(ctx, websiteID)
	if err != nil {
		return shop.Overview{}, err
	}
	return shop.OverviewOf(orders, products, now), nil
}
