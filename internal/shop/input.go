package shop

// ProductInput is one product the way an operator types it: the product form
// on the admin screen, or the arguments of an assistant's tool call through the
// admin handler. Both go through the same validation, so they are one shape.
//
// Prices and stock travel as text, not as numbers. "49,50" is a price people
// write every day, and an empty stock field means "not tracked", which a number
// cannot say. The parsing happens once, in the admin's validation.
//
// It lives here rather than in the admin package because the assistant's tools
// have to be able to name it, and they cannot import the admin package.
type ProductInput struct {
	ID           int64
	Slug         string
	Title        string
	Subtitle     string
	Markdown     string
	SKU          string
	Price        string
	TaxBP        int
	StockText    string
	WeightGrams  int
	DeliveryNote string
	Status       string
	FeaturedID   int64
	// Terms are the categories, comma-separated, the way the form edits them.
	Terms string
}

// SettingsInput is a website's shop configuration the way an operator types
// it. Amounts are text for the same reason as ProductInput's price; an empty
// ShippingFreeAt means there is no free-shipping threshold at all.
type SettingsInput struct {
	ShopBase       string
	Currency       string
	ShippingGross  string
	ShippingFreeAt string
	ShippingTaxBP  int
	PriceDisplay   string
	VATExempt      bool
	VATNumber      string
	ReturnPolicy   string
	OrderEmail     string
	PaymentDetails string
}
