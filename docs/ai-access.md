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
| `websites_auflisten` | the websites of this installation | no |
| `seiten_auflisten` | pages of a website, without their text | no |
| `seite_lesen` | one page including its Markdown | no |
| `seiten_durchsuchen` | full-text search, drafts included | no |
| `medien_auflisten` | images and files with a ready-made Markdown reference | no |
| `felder_auflisten` | the website's own fields including groups | no |
| `seite_anlegen` | a new page, always as a draft; accepts own fields | yes |
| `seite_aendern` | title, text or individual fields; the status stays | yes |
| `seite_veroeffentlichen` | make public, or take back | yes |

A read-only key is not even shown the three writing tools — and is refused if it
calls them anyway. A page made of blocks cannot be overwritten with Markdown; the
assistant is told why, and that this is done in the admin.
