# AI access

An operator can connect their own AI assistant to the running CMS and write
content with it. The direction matters: **this server never calls an AI.** There
is no provider credential here, nothing is fetched at runtime, and the rule that
nothing comes from third parties while the program runs is untouched. The
assistant runs wherever the operator runs it, and signs in from outside.

## The endpoint

The protocol is MCP — the one Claude, ChatGPT and the editors speak: JSON-RPC 2.0
at `POST /ai`, authenticated with `Authorization: Bearer <key>`.

The address sits outside CSRF protection and outside the session, because no
browser signs in here; a form cannot set that header, so the gap CSRF otherwise
protects against does not exist.

## Keys

Keys are issued in the admin under *AI access*. A key is visible exactly once —
only its SHA-256 digest is stored, the same discipline as for an invitation link.
It is valid either for all websites or for one, either read-only or read-write,
either indefinitely or until a date. When it was last used is shown in the list;
that is the trace by which you notice a key being used you did not expect.

Issuing a key is one of the four actions that ask for the password again — see
[security](security.md#the-password-again-before-the-irreversible).

## Tools

Whatever an assistant does, it does through the same stores as the admin: the
same address rules, the same versions, the same validation. Two rules sit on top:

- **Anything new is always a draft.** Publishing happens only through its own,
  explicit call. An assistant that publishes by accident puts something
  half-finished on the web, and it is noticed only once somebody has read it.
- **Changing is not publishing.** A published page stays published, a draft stays
  a draft, and the previous state is kept as a version and can be brought back.

| Tool | What it does | Writes |
|---|---|---|
| `list_websites` | the websites of this installation | no |
| `list_pages` | pages of a website, without their text | no |
| `read_page` | one page including its Markdown | no |
| `search_pages` | full-text search, drafts included | no |
| `list_media` | images and files with a ready-made Markdown reference | no |
| `list_fields` | the website's own fields including groups | no |
| `create_page` | a new page, always as a draft; accepts own fields | yes |
| `update_page` | title, text or individual fields; the status stays | yes |
| `publish_page` | make public, or take back | yes |

A read-only key is not even shown the three writing tools — and is refused if it
calls them anyway. A page made of blocks cannot be overwritten with Markdown; the
assistant is told why, and that this is done in the admin.

## The vocabulary changed in 2.0

Up to 1.x every tool name, every argument and every answer key was German:
`seite_anlegen`, `"titel"`, `"zustand": "entwurf"`. Since 2.0 they are English —
`create_page`, `"title"`, `"status": "draft"` — for the same reason the template
contract is: the words a machine reads are part of the codebase, not part of what
an operator writes.

This is a **breaking change**. A stored prompt or a script that names a tool or
an argument by its German name stops working, and says so: the server answers
`there is no tool "seite_anlegen"` rather than doing something unexpected. An
assistant that asks for the tool list — which is how MCP is meant to be used —
notices nothing at all.

What did **not** change is anything an operator typed: a field key stays
`preis`, a content type stays `produkt`, and a field kind stays `mehrfachauswahl`
in the answer of `list_fields`. Those are values in the database, not vocabulary
of the protocol.
