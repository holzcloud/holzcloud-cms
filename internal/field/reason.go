package field

import "github.com/holzcloud/holzcloud-cms/internal/i18n"

// Reason is why a value was refused: a sentence the catalogue can hold, plus
// the values that fill it in.
//
// It exists because the finished sentence could not be translated. Check used
// to return `d.Label + " muss eine Zahl sein."` — a string built by
// concatenation, at run time, out of the operator's own label and a German
// literal. Three things are wrong with that at once, and only the third is
// obvious:
//
//   - tools/i18n cannot see it. The collector takes a string literal at a known
//     argument index of a known function; a `+` between two expressions is an
//     *ast.BinaryExpr and is skipped without a word. So the sentence is not
//     "untranslated", it is *unreported*, and the gate reads `0 offen` over it.
//   - It could not become a key even by hand. The label is the operator's
//     ("Preis", "Wurfdatum", "Artikelnummer"); a catalogue keyed on the finished
//     sentence would need one entry per label anybody ever typed.
//   - And so, measured on 2026-09-08 with the admin set to English, a form
//     answered "Preis muss eine Zahl sein." next to fields whose kind names
//     were translated.
//
// Splitting the sentence in two fixes all three: the format is a literal and is
// collected, the label travels as an argument and is never translated, and the
// sentence is assembled where the language of the reader is known.
//
// Args may itself hold a Reason. That is how the group case works — "%s, Zeile
// %d: %s" wraps the reason of one sub-field — and Text resolves it recursively,
// so a nested reason is translated too rather than printed as a Go struct.
type Reason struct {
	// Format is the sentence, marked with i18n.N where it is minted so that it
	// reaches the catalogue.
	Format string
	// Args fill the format's verbs. An arg is operator data (a label, a bound,
	// a count) or a nested Reason.
	Args []any
}

// Empty reports whether there is no reason — the value is fine.
//
// The zero Reason is the "no problem" value, so a caller writes
// `if r := Check(d, v); !r.Empty()` exactly where it used to write `!= ""`.
func (r Reason) Empty() bool { return r.Format == "" }

// Text renders the reason in one language.
//
// Nested reasons are rendered first, in the same language, which is the whole
// point of allowing them: a group's row message and the sub-field message
// inside it must not end up in two different languages on one line.
func (r Reason) Text(lang string) string {
	if r.Empty() {
		return ""
	}
	args := make([]any, len(r.Args))
	for i, a := range r.Args {
		if nested, ok := a.(Reason); ok {
			args[i] = nested.Text(lang)
			continue
		}
		args[i] = a
	}
	return i18n.Tf(lang, r.Format, args...)
}

// String is the reason in the source language — German, which is what a Reason
// says when nobody has said who is reading.
//
// It exists so a Reason prints as its sentence in a test failure and in a log
// line rather than as a Go struct. It is deliberately NOT what a screen calls:
// a screen knows its reader and calls Text with that reader's language. If this
// method starts turning up in handlers, somebody has thrown that away.
func (r Reason) String() string { return r.Text(i18n.Source) }

// reasonf builds a Reason. The format must be written as i18n.N("…") at the
// call site — not here — because the collector reads the literal where it
// stands and a format handed in through a variable is invisible again.
func reasonf(format string, args ...any) Reason {
	return Reason{Format: format, Args: args}
}
