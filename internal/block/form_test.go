package block

import "testing"

// "+ Element" between two blocks puts the new one there, not at the end.
func TestAddInsertsAtAPosition(t *testing.T) {
	blocks := []Block{{Type: TypeText, Markdown: "a"}, {Type: TypeText, Markdown: "b"}}
	got := Apply(blocks, ActionAdd+":"+TypeQuote+"@1", Builtin)
	if len(got) != 3 || got[1].Type != TypeQuote || got[0].Markdown != "a" || got[2].Markdown != "b" {
		t.Fatalf("got %+v", got)
	}
	got = Apply(got, ActionAdd+":"+TypeQuote+"@99", Builtin)
	if len(got) != 4 || got[3].Type != TypeQuote {
		t.Fatalf("a position past the end did not append: %+v", got)
	}
	got = Apply(got, ActionAdd+":nonsense@0", Builtin)
	if len(got) != 4 {
		t.Fatalf("an unknown kind was inserted: %+v", got)
	}
}
