# Quick 261031-pqo: Plugin screen marks its own rail entry; shorter menu label

1. `internal/admin/plugin.go`: ActiveNav = `plugin:<screen URL>` instead of `plugins` (which folded into Settings and highlighted Plugins). URL built by new shared `web.PluginScreenURL` (also used by `pluginNavLinks`) so rail link and handler cannot drift.
2. Test: `TestAPluginScreenMarksItsOwnEntryNotSettings` (internal/web).
3. `plugins/kontaktformular/plugin.json`: label "Forms & messages" -> "Messages" (de Nachrichten, es Mensajes, fr Messages, it Messaggi), version 1.5.1; repack zip via `go run ./tools/wasm kontaktformular`.
