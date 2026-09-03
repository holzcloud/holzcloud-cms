package tmplmgr

import "time"

// Template represents a site template stored on disk.
type Template struct {
	ID        int64
	Name      string
	Slug      string
	IsBuiltin bool
	CreatedAt time.Time
}

// WebsiteTemplate represents the association between a website and a template.
type WebsiteTemplate struct {
	ID         int64
	WebsiteID  int64
	TemplateID int64
	IsActive   bool
	CreatedAt  time.Time
	// Joined fields
	TemplateName string
	TemplateSlug string
}
