package admin

import (
	"os"
	"sync"
	"testing"

	"github.com/holzcloud/holzcloud-cms/internal/web"
)

// The admin world is parsed once per installed language, a few hundred
// milliseconds each and several times that under the race detector. The tests
// of this package used to pay it for every handler they built. They share one
// parse instead, per test binary.
//
// A render only reads the parsed sets, so sharing is safe for as long as
// nothing replaces them. The language handlers do: they call Reload on the set
// their Handler holds. A test that reaches one of them, or that changes the
// folder of installed languages, takes privateAdminTemplates instead, so the
// set every later test sees is the one it started with.
const adminTemplatesDir = "../../cmd/holzcloud/templates/admin"

var (
	sharedOnce      sync.Once
	sharedTemplates *web.PageTemplates
	sharedErr       error
)

// sharedAdminTemplates returns the parse every ordinary admin test renders
// with. A template that does not parse fails every test that asks for it.
func sharedAdminTemplates(tb testing.TB) *web.PageTemplates {
	tb.Helper()
	sharedOnce.Do(func() {
		sharedTemplates, sharedErr = web.ParseAdminTemplates(os.DirFS(adminTemplatesDir))
	})
	if sharedErr != nil {
		tb.Fatalf("ParseAdminTemplates: %v", sharedErr)
	}
	return sharedTemplates
}

// privateAdminTemplates parses a set for one test alone, for a test that
// reloads or otherwise replaces what its Handler holds.
func privateAdminTemplates(tb testing.TB) *web.PageTemplates {
	tb.Helper()
	templates, err := web.ParseAdminTemplates(os.DirFS(adminTemplatesDir))
	if err != nil {
		tb.Fatalf("ParseAdminTemplates: %v", err)
	}
	return templates
}
