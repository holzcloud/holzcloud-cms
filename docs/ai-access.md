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

A key has one of three levels:

| Level | May | Where it is made |
|---|---|---|
| `read` | read everything the key's website(s) hold | admin (*AI access*) or command line |
| `content` | also write: pages, media, menus, design, shop … | admin (*AI access*) or command line |
| `admin` | also manage the installation: users, keys, plugins, languages, brand | **command line only** |

A key is visible exactly once — only its SHA-256 digest is stored, the same
discipline as for an invitation link. A `read` or `content` key is valid for all
websites or for one; an `admin` key always reaches every website, because a user
or a plugin belongs to the installation and not to one site. Keys may expire.
When a key was last used is shown in the list; that is the trace by which you
notice a key being used you did not expect.

### Without the web interface

Everything can be done without ever signing in to the admin. On the server:

```bash
holzcloud ai key create -name "Claude on my laptop" -level admin
holzcloud ai key list
holzcloud ai key revoke 3
```

`create` prints the key once. From there the assistant can create websites,
invite people, issue further `read` and `content` keys and everything else the
admin can do. An `admin` key is never issued through the connection itself and
never on a screen: whoever holds the server holds the installation anyway, and
nobody else should be able to mint one. `-website <id>` limits a key to one
website, `-days <n>` lets it expire.

Issuing a key in the admin is one of the actions that ask for the password again
— see [security](security.md#the-password-again-before-the-irreversible).

## Tools

Since 2.7 the connection can do everything the admin can — about 140 tools. An
assistant asks for the list (`tools/list`) and gets exactly the tools its key may
use: a `read` key sees no writing tool, a `content` key no admin tool, and a call
past that is refused anyway.

Whatever an assistant does, it does through the same code as the admin screens:
the same address rules, the same versions, the same validation, the same upload
checks, the same activity log — where a screen does more than one store call,
the tool calls the very function the screen calls (`internal/admin/ops*.go`).
Every change appears in the activity log under `KI: <name of the key>`. Rules on
top:

- **Anything new is always a draft.** Publishing happens only through its own,
  explicit call.
- **Changing is not publishing.** A published page stays published, a draft stays
  a draft, and the previous state is kept as a version and can be brought back.
- **Deleting needs `confirm: true`.** Every tool that removes something refuses
  without it.
- **Unknown parameters are refused.** A misspelt argument is an error that names
  it, never a silently ignored value.
- **Files arrive as base64** in the call (`upload_media`, `upload_template`,
  `install_plugin`, a logo). The server never fetches a URL.

| Area | Tools |
|---|---|
| Websites & design | `list_websites` `get_website` `update_website` `create_website`\* `delete_website`\* `add_domain`\* `remove_domain`\* `set_primary_domain`\* `launch_checklist` `translation_matrix` `list_templates` `activate_template` `upload_template`\* `delete_template`\* `get_template_spec` `get_design` `set_design`\* `reset_design`\* `get_wording` `set_wording` |
| Pages | `list_pages` `read_page` `search_pages` `create_page` `update_page` `publish_page` `get_page_settings` `set_page_slug` `set_page_kind` `set_page_terms` `set_page_seo` `set_page_schedule` `set_page_protection` `list_block_kinds` `get_page_blocks` `set_page_blocks` `insert_block` `list_revisions` `read_revision` `restore_revision` `label_revision` `duplicate_page` `list_translations` `create_translation` `create_share_link` `set_review` `bulk_pages` `list_trash` `trash_page` `restore_page` `purge_page`\* |
| Media & albums | `list_media` `get_media_library` `list_media_collection` `get_media` `upload_media` `update_media` `set_media_focus` `crop_media` `restore_media_original` `delete_media` `list_albums` `get_album` `create_album` `rename_album` `delete_album` `add_album_images` `update_album_image` `remove_album_image` `move_album_image` |
| Structure | `list_menus` `get_menu` `create_menu` `update_menu` `delete_menu` `list_terms` `rename_term` `delete_term` `list_snippets` `get_snippet` `create_snippet` `update_snippet` `delete_snippet` `list_redirects` `create_redirect` `delete_redirect` `check_links` `list_fields` `create_field`\* `update_field`\* `delete_field`\* `move_field`\* `list_kinds`\* `create_kind`\* `update_kind`\* `delete_kind`\* `move_kind`\* `manage_block_kinds`\* `create_block_kind`\* `update_block_kind`\* `delete_block_kind`\* `move_block_kind`\* |
| Shop | `shop_overview` `list_products` `get_product` `create_product` `update_product` `delete_product` `list_orders` `get_order` `set_order_status` `recheck_payment` `resend_order_mail` `get_shop_settings`\* `update_shop_settings`\* |
| Installation | `key_info` and, admin keys only: `list_users` `get_user` `invite_user` `update_user` `create_password_reset_link` `create_invitation_link` `end_user_sessions` `disable_user_two_factor` `delete_user` `list_ai_keys` `create_ai_key` `revoke_ai_key` `list_plugins` `install_plugin` `enable_plugin` `disable_plugin` `set_plugin_websites` `remove_plugin` `get_mail_status` `retry_mail` `send_test_mail` `list_languages` `install_language` `remove_language` `get_branding` `update_branding` `read_activity_log` |

\* admin key only — the same split as the admin, where these screens are for
administrators.

`create_ai_key` issues `read` and `content` keys only; `revoke_ai_key` refuses
the key that calls it, so an assistant cannot cut its own connection halfway
through a task.

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
