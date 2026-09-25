package activity

// The shop's action strings. In a file of their own beside entry.go so that
// the list there stays the one for the rest of the admin; the same rule holds
// for these: a name is a filter contract, renaming one loses the rows already
// written, adding is free.
//
// The shop screens themselves log nothing yet. The assistant's tools do,
// because a change made without anybody signed in has to be traceable to the
// key that made it.
const (
	ActionProductCreate = "product.create"
	ActionProductUpdate = "product.update"
	ActionProductDelete = "product.delete"

	ActionOrderStatus    = "order.status"
	ActionOrderPayment   = "order.payment_check"
	ActionOrderMailRetry = "order.mail_retry"

	ActionShopSettingsSave = "shop.settings_save"
)
