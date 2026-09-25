package shop

import (
	"time"

	"github.com/holzcloud/holzcloud-cms/internal/money"
)

// The shop's front door as numbers: what came in this month, what is open,
// what is running out, and how the weeks went. Everything is computed from the
// orders and products that already exist — there is no counter kept anywhere
// else that could drift.
//
// The admin's overview screen and the assistant's shop_overview tool both read
// it from here, so the two cannot disagree about what "open" means.

// LowStockBelow is where a product counts as nearly sold out. Three, because a
// workshop that sells a stool a week wants to know a fortnight ahead.
const LowStockBelow = 3

// PaymentReminderAfter is how long an unpaid order waits before it is named as
// something to chase.
const PaymentReminderAfter = 3 * 24 * time.Hour

// OverviewWeeks is how many weeks the revenue chart covers.
const OverviewWeeks = 8

// What a task on the overview is about.
const (
	TaskPayment  = "payment"
	TaskDispatch = "dispatch"
	TaskStock    = "stock"
)

// Overview is the shop at a glance.
type Overview struct {
	ThisMonth money.Amount
	LastMonth money.Amount
	// RevenueChange is this month against last month in percent, valid only
	// when HasPrevious is set.
	RevenueChange int
	HasPrevious   bool

	OpenOrders     int
	AwaitPayment   int
	LowStock       int
	ProductsOnline int
	ProductDrafts  int

	Tasks []Task
	Weeks []WeekRevenue
	// Best is the best week's revenue, BestWeek its ISO week number; both
	// zero when no week had any.
	Best     money.Amount
	BestWeek int
	// HasOrders is false for a shop that has never taken an order.
	HasOrders bool
}

// Task is one thing to be done, in raw form: the screen and the tool each word
// it for their own reader.
type Task struct {
	Kind string
	// Warn marks what is overdue rather than merely next.
	Warn bool

	OrderNumber string
	DaysWaiting int

	ProductID    int64
	ProductTitle string
	Stock        int
}

// WeekRevenue is one bar of the chart. Weeks run Monday to Sunday.
type WeekRevenue struct {
	Start  time.Time
	Week   int
	Amount money.Amount
	// Percent is the bar's height against the best week, 0–100.
	Percent int
	Current bool
}

// counts reports whether an order is money that came in: paid or already on
// its way. A new order may never be paid; a cancelled one was not.
func counts(o *Order) bool {
	return o.Status == OrderPaid || o.Status == OrderShipped
}

// OverviewOf computes the overview from a website's orders and products as of
// now. The orders are expected newest first, as OrderStore.List returns them;
// the tasks come out in that order, followed by the products.
func OverviewOf(orders []*Order, products []*Product, now time.Time) Overview {
	ov := Overview{HasOrders: len(orders) > 0}

	monthStart := time.Date(now.Year(), now.Month(), 1, 0, 0, 0, 0, now.Location())
	prevStart := monthStart.AddDate(0, -1, 0)

	weekStart := func(t time.Time) time.Time {
		d := time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
		offset := (int(d.Weekday()) + 6) % 7
		return d.AddDate(0, 0, -offset)
	}
	current := weekStart(now)
	first := current.AddDate(0, 0, -7*(OverviewWeeks-1))
	weekly := make([]money.Amount, OverviewWeeks)

	for _, o := range orders {
		created := o.CreatedAt.In(now.Location())
		if counts(o) {
			switch {
			case !created.Before(monthStart):
				ov.ThisMonth += o.Totals.TotalGross
			case !created.Before(prevStart):
				ov.LastMonth += o.Totals.TotalGross
			}
			if !created.Before(first) {
				i := int(weekStart(created).Sub(first).Hours() / (24 * 7))
				if i >= 0 && i < OverviewWeeks {
					weekly[i] += o.Totals.TotalGross
				}
			}
		}
		switch o.Status {
		case OrderNew:
			ov.OpenOrders++
			if o.PaymentStatus == PaymentOpen {
				ov.AwaitPayment++
				if waited := now.Sub(o.CreatedAt); waited > PaymentReminderAfter {
					ov.Tasks = append(ov.Tasks, Task{
						Kind: TaskPayment, Warn: true,
						OrderNumber: o.Number, DaysWaiting: int(waited.Hours() / 24),
					})
				}
			}
		case OrderPaid:
			ov.OpenOrders++
			ov.Tasks = append(ov.Tasks, Task{Kind: TaskDispatch, OrderNumber: o.Number})
		}
	}

	for _, p := range products {
		if p.Status == StatusPublished {
			ov.ProductsOnline++
		} else {
			ov.ProductDrafts++
		}
		if p.Status == StatusPublished && p.Stock != nil && *p.Stock < LowStockBelow {
			ov.LowStock++
			ov.Tasks = append(ov.Tasks, Task{
				Kind: TaskStock, Warn: *p.Stock == 0,
				ProductID: p.ID, ProductTitle: p.Title, Stock: *p.Stock,
			})
		}
	}

	if ov.LastMonth > 0 {
		ov.HasPrevious = true
		ov.RevenueChange = int((int64(ov.ThisMonth) - int64(ov.LastMonth)) * 100 / int64(ov.LastMonth))
	}

	for _, a := range weekly {
		if a > ov.Best {
			ov.Best = a
		}
	}
	for i, a := range weekly {
		start := first.AddDate(0, 0, 7*i)
		_, wk := start.ISOWeek()
		pct := 0
		if ov.Best > 0 {
			pct = int(int64(a) * 100 / int64(ov.Best))
		}
		if a == ov.Best && ov.Best > 0 {
			ov.BestWeek = wk
		}
		ov.Weeks = append(ov.Weeks, WeekRevenue{
			Start: start, Week: wk, Amount: a, Percent: pct, Current: i == OverviewWeeks-1,
		})
	}
	return ov
}
