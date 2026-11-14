package activity

// The privacy actions of Phase 33.2: answering a person's request about their
// data, and the retention that deletes what is no longer needed. Declared
// together here so that the plans of the phase add no constants of their own.
//
// None of them ever carries the person's e-mail address or any other value of
// the records it touched. The log outlives an erasure — it is the record that
// the erasure happened — and a log that kept the address would keep exactly
// what was erased.
const (
	// ActionPrivacyExport is a person's data downloaded from the Privacy
	// screen; the metadata carries the number of records.
	ActionPrivacyExport = "privacy.export"
	// ActionPrivacyAnswer is a privacy request answered; the metadata carries
	// the request id and how it was answered.
	ActionPrivacyAnswer = "privacy.answer"
	// ActionPrivacyRequestAdd is a request entered by hand, one that arrived
	// by letter or telephone; the metadata carries the request id and kind.
	ActionPrivacyRequestAdd = "privacy.request_add"
	// ActionPrivacyErase is a person's data erased; the metadata carries the
	// number of records per source and what was kept.
	ActionPrivacyErase = "privacy.erase"
	// ActionPrivacyCorrect is a record corrected; the metadata carries the
	// source, the record key and the names of the fields, never their values.
	ActionPrivacyCorrect = "privacy.correct"
	// ActionPrivacyRetention is a run of the retention job; the metadata
	// carries how many records of each kind it deleted.
	ActionPrivacyRetention = "privacy.retention"
	// ActionPrivacyPeriods is a website's retention periods changed; the
	// metadata carries the kinds and their new periods.
	ActionPrivacyPeriods = "privacy.periods"
)
