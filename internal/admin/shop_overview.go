package admin

import (
	"net/http"
	"strconv"
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/money"
	"github.com/holzcloud/holzcloud-cms/internal/shop"
	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The shop's front door: four numbers, what is to be done, and how the weeks
// went. Everything on it is computed from the orders and products that already
// exist — there is no counter kept anywhere else that could drift.

// lowStockBelow is where a product counts as nearly sold out. Three, because a
// workshop that sells a stool a week wants to know a fortnight ahead.
const lowStockBelow = 3

// paymentReminderAfter is how long an unpaid order waits before it is named as
// something to chase.
const paymentReminderAfter = 3 * 24 * time.Hour

// overviewWeeks is how many weeks the chart shows.
const overviewWeeks = 8

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

// counts reports whether an order is money that came in: paid or already on
// its way. A new order may never be paid; a cancelled one was not.
func counts(o *shop.Order) bool {
	return o.Status == shop.OrderPaid || o.Status == shop.OrderShipped
}

// HandleShopOverview shows the overview of one website's shop.
func (h *Handler) HandleShopOverview(w http.ResponseWriter, r *http.Request) error {
	ws, ok, err := h.shopWebsite(w, r)
	if err != nil || !ok {
		return err
	}
	ctx := r.Context()
	cur := money.CurrencyFor(ws.Currency)
	now := time.Now()

	var orders []*shop.Order
	if h.orders != nil {
		// A year of a workshop's orders; the chart needs eight weeks and the
		// month two, so this is plenty and still one query.
		if orders, err = h.orders.List(ctx, ws.ID, 1000); err != nil {
			return err
		}
	}
	products, err := h.products.List(ctx, ws.ID)
	if err != nil {
		return err
	}

	data := shopOverviewData{
		LayoutData: web.NewLayoutData(r, h.sm, web.Titlef(r, "Shop – %s", ws.Name)),
		HasOrders:  len(orders) > 0,
	}
	data.ActiveNav = "shop"
	data.CurrentWebsite = ws

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	prevStart := monthStart.AddDate(0, -1, 0)
	var thisMonth, lastMonth money.Amount

	// Weeks run Monday to Sunday; the last bar is the current week.
	weekStart := func(t time.Time) time.Time {
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		offset := (int(d.Weekday()) + 6) % 7
		return d.AddDate(0, 0, -offset)
	}
	current := weekStart(now)
	first := current.AddDate(0, 0, -7*(overviewWeeks-1))
	weekly := make([]money.Amount, overviewWeeks)

	for _, o := range orders {
		created := o.CreatedAt.In(now.Location())
		if counts(o) {
			switch {
			case !created.Before(monthStart):
				thisMonth += o.Totals.TotalGross
			case !created.Before(prevStart):
				lastMonth += o.Totals.TotalGross
			}
			if !created.Before(first) {
				i := int(weekStart(created).Sub(first).Hours() / (24 * 7))
				if i >= 0 && i < overviewWeeks {
					weekly[i] += o.Totals.TotalGross
				}
			}
		}
		switch o.Status {
		case shop.OrderNew:
			data.OpenOrders++
			if o.PaymentStatus == shop.PaymentOpen {
				data.AwaitPayment++
				if now.Sub(o.CreatedAt) > paymentReminderAfter {
					data.Tasks = append(data.Tasks, shopTask{
						Kind: web.T(r, "Payment"), Warn: true,
						Text:   web.Titlef(r, "Order %s has been waiting for its payment for %d days", o.Number, int(now.Sub(o.CreatedAt).Hours()/24)),
						URL:    "/admin/websites/" + strconv.FormatInt(ws.ID, 10) + "/bestellungen/" + o.Number,
						Action: web.T(r, "Open"),
					})
				}
			}
		case shop.OrderPaid:
			data.OpenOrders++
			data.Tasks = append(data.Tasks, shopTask{
				Kind:   web.T(r, "Dispatch"),
				Text:   web.Titlef(r, "Order %s is paid and waiting to be sent", o.Number),
				URL:    "/admin/websites/" + strconv.FormatInt(ws.ID, 10) + "/bestellungen/" + o.Number,
				Action: web.T(r, "Delivery note"),
			})
		}
	}

	for _, p := range products {
		if p.Status == shop.StatusPublished {
			data.ProductsOnline++
		} else {
			data.ProductDrafts++
		}
		if p.Status == shop.StatusPublished && p.Stock != nil && *p.Stock < lowStockBelow {
			data.LowStock++
			data.Tasks = append(data.Tasks, shopTask{
				Kind: web.T(r, "Stock"), Warn: *p.Stock == 0,
				Text:   web.Titlef(r, "%s: %d left", p.Title, *p.Stock),
				URL:    "/admin/websites/" + strconv.FormatInt(ws.ID, 10) + "/produkte/" + strconv.FormatInt(p.ID, 10),
				Action: web.T(r, "Restock"),
			})
		}
	}

	data.Revenue = cur.Format(thisMonth)
	if lastMonth > 0 {
		data.HasPrevious = true
		data.RevenueChange = int((int64(thisMonth) - int64(lastMonth)) * 100 / int64(lastMonth))
	}

	var best money.Amount
	for _, a := range weekly {
		if a > best {
			best = a
		}
	}
	bestLabel := ""
	for i, a := range weekly {
		start := first.AddDate(0, 0, 7*i)
		_, wk := start.ISOWeek()
		label := web.Titlef(r, "Wk %d", wk)
		pct := 0
		if best > 0 {
			pct = int(int64(a) * 100 / int64(best))
		}
		if a == best && best > 0 {
			bestLabel = label
		}
		data.Weeks = append(data.Weeks, shopWeek{Label: label, Amount: cur.Format(a), Percent: pct, Last: i == overviewWeeks-1})
	}
	if best > 0 {
		data.BestWeek = web.Titlef(r, "Best week %s: %s", bestLabel, cur.Format(best))
	}

	return web.RenderAdmin(w, h.templates, r, "shop_overview", data)
}
