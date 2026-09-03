// Package activity keeps the record of what was done in the administration.
//
// It answers one question: who changed this, and when. That question comes up
// when several people share an installation and something is suddenly not the
// way it was — and it is worth very little if the answer only survives as long
// as the account that gave it. So the acting address is copied into every row
// and the foreign keys give way instead of taking the row with them.
//
// Writing is deliberately one-way: Store.Log never returns an error. A record
// that cannot be written is worth a line in the log, but it must not turn a
// working save into a 500 — the audit trail serves the work, not the other way
// round.
package activity

import "time"

// Entry is one row of activity_log.
//
// UserID and WebsiteID are pointers because both columns fall to NULL when the
// parent row goes, and because some events have neither: a failed sign-in
// happens before there is a user.
//
// ActorEmail is a copy taken at write time. It outlives the account and saves
// the list view a lookup per row.
//
// Metadata goes through the deny-list in Store.Log before it is written, so a
// caller may hand over form values without sorting them first.
type Entry struct {
	ID         int64
	UserID     *int64
	ActorEmail string
	Action     string
	EntityType string
	EntityID   int64
	WebsiteID  *int64
	Metadata   map[string]any
	CreatedAt  time.Time
}

// The action strings. Call sites use these instead of literals, because a
// typo in a literal does not fail anywhere — it just quietly writes a row that
// no filter will ever match again.
//
// The names are the filter contract: renaming one makes the rows already in
// the database unfindable. Adding is free.
const (
	ActionAuthLoginSuccess = "auth.login_success"
	ActionAuthLoginFail    = "auth.login_fail"
	ActionAuthLogout       = "auth.logout"
	ActionAuthTokenCreate  = "auth.token_create"
	ActionAuthTokenConsume = "auth.token_consume"

	ActionPageCreate          = "page.create"
	ActionPageUpdate          = "page.update"
	ActionPagePublish         = "page.publish"
	ActionPageUnpublish       = "page.unpublish"
	ActionPageDelete          = "page.delete"
	ActionPageRevisionRestore = "page.revision.restore"

	ActionUserCreate = "user.create"
	ActionUserUpdate = "user.update"
	ActionUserDelete = "user.delete"

	ActionTemplateActivate = "template.activate"

	ActionDomainAdd    = "domain.add"
	ActionDomainRemove = "domain.remove"

	// Das Design heisst auf diesem Stamm design und nicht theme; die
	// Zeichenkette folgt dem Paketnamen, damit ein Filter sich raten lässt.
	ActionDesignSave = "design.save"

	ActionActivityPurge = "activity.purge"
)
