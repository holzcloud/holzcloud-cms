#!/usr/bin/env python3
"""Set a project's released version as one value of holzcloud.ch and read it back.

holzcloud.ch is website 1 of the holzcloud-CMS. Each project has one page
per language (de, fr, it, rm, en), and those pages show the current version
of the project. A release workflow calls this script after its tag exists,
so the site never shows an older version than the one that was released.

How the number gets onto the pages: since 0.1.5 the CMS keeps values per
website, and it replaces a placeholder [[wert:key]] in the page text with the
value's text when it delivers the page - in a sentence ("Alpha
[[wert:version-holzcloud-cms]]: ...") and in a link address
(".../releases/tag/v[[wert:version-holzcloud-cms]]") alike. The pages carry
the placeholder, not the number, so a release changes one value and no
page. This script does exactly that, for every project:

  1. read the project's value (list_values);
  2. set it to the released version (set_value);
  3. read it back, and fail if it does not say what was set.

The keys:

  project            value key on holzcloud.ch
  holzcloud-cms      version-holzcloud-cms
  holzkube-manager   version-holzkube-manager
  hauscloud          version-hauscloud      (also: hauscloud.ch)
  holzice            version-holzbar        (also: holzIce, holzBar, holzbar)

A value that already holds a newer version is left alone (--allow-older
overrides), an equal one is left as it is, and a missing one is created. A
beta or a release candidate is not a plain X.Y.Z release and is skipped.
A CMS older than 0.1.5 has no values; it answers the first call with
"there is no tool" (JSON-RPC -32601), and the script prints a notice and
sets nothing. With --site, the published pages that name the value are
fetched afterwards and a page that does not show the number is a warning
(a cache may lag behind), not a failure.

Usage:
  CMS_TOKEN=... cms-version.py --project holzkube-manager --version v0.2.3
  cms-version.py --project holzice --version 0.0.6 --dry-run
  cms-version.py --project holzcloud-cms --version v0.1.5 --website 2 \\
      --endpoint http://localhost:8080/ai --site http://localhost:8080

The token is a holzcloud-CMS AI key of level "content", limited to the
website (made in the CMS admin under "KI-Zugang"). --website defaults to the
environment variable CMS_WEBSITE, else 1. Without CMS_TOKEN the script
prints a notice and exits 0, so a release keeps working until the secret
exists. Exit codes: 0 done or skipped, 1 refused or failed. A failure is one
GitHub ::error:: line, never a traceback: a connection that is refused,
reset, dropped or cut off is tried three times, and an answer that is not
the JSON-RPC object the CMS sends (a proxy's or a maintenance page's HTML,
say) is named as such.

The endpoint is the CMS's MCP address: JSON-RPC 2.0, POST, bearer key,
stateless - one tools/call per request. Python standard library only.
"""
from __future__ import annotations

import argparse
import http.client
import json
import os
import re
import sys
import time
import urllib.error
import urllib.parse
import urllib.request

DEFAULT_ENDPOINT = "https://holzcloud.ch/ai"
DEFAULT_SITE = "https://holzcloud.ch"
DEFAULT_WEBSITE = 1

# The JSON-RPC code a CMS answers for a tool it does not have. A CMS older
# than 0.1.5 has no list_values, so this is how such a server is recognised.
NO_SUCH_TOOL = -32601

PLAIN = re.compile(r"\d+\.\d+\.\d+")

# Each project and the key of its value on holzcloud.ch. The pages name the
# value as [[wert:<key>]]; a different key here shows nothing on them.
PROJECTS = {
    "holzcloud-cms": "version-holzcloud-cms",
    "holzkube-manager": "version-holzkube-manager",
    "hauscloud": "version-hauscloud",
    # The repository was renamed from holzIce to holzBar on 2026-10-02.
    "holzice": "version-holzbar",
}
# The names the release workflows use, so a caller can pass its repository.
ALIASES = {"hauscloud.ch": "hauscloud", "holzIce": "holzice", "holzBar": "holzice", "holzbar": "holzice"}


class Refused(Exception):
    """Something is not as expected; the run fails."""


class RpcError(Refused):
    """The CMS answered with a JSON-RPC error; code says which."""

    def __init__(self, tool: str, code, message: str):
        super().__init__(f"{tool}: {message}")
        self.tool = tool
        self.code = code
        self.message = message


class ToolError(Refused):
    """The tool answered with an error (isError) rather than a result."""

    def __init__(self, tool: str, message: str):
        super().__init__(f"{tool}: {message}")
        self.message = message


# ---------------------------------------------------------------- the CMS


class CMS:
    def __init__(self, endpoint: str, token: str, timeout: float = 30.0):
        self.endpoint = endpoint
        self.token = token
        self.timeout = timeout
        self._id = 0

    def call(self, tool: str, arguments: dict):
        """One tools/call. Returns the decoded JSON the tool answered."""
        self._id += 1
        body = json.dumps({
            "jsonrpc": "2.0", "id": self._id, "method": "tools/call",
            "params": {"name": tool, "arguments": arguments},
        }).encode()
        req = urllib.request.Request(self.endpoint, data=body, method="POST", headers={
            "Authorization": "Bearer " + self.token,
            "Content-Type": "application/json",
            "Accept": "application/json",
            "User-Agent": "holzcloud-cms-version/2",
        })
        # Retried: a 5xx, and every way the connection can fail on the way -
        # refused, reset, dropped before the answer (RemoteDisconnected), cut
        # off in the middle of the body (IncompleteRead), timed out. The first
        # three are OSError (URLError is one too), the last two are
        # http.client's own. Not retried: an answer that arrived whole and is
        # wrong, below - asking again gets the same proxy page.
        last = None
        for attempt in range(3):
            try:
                with urllib.request.urlopen(req, timeout=self.timeout) as res:
                    raw = res.read()
                break
            except urllib.error.HTTPError as e:
                if e.code in (401, 403):
                    raise Refused(f"the CMS refused the key (HTTP {e.code}); is CMS_TOKEN "
                                  "a valid content key for this website?")
                last = e
                if e.code < 500:
                    raise Refused(f"the CMS answered HTTP {e.code} to {tool}")
            except (OSError, http.client.HTTPException) as e:
                last = e
            time.sleep(2 * (attempt + 1))
        else:
            raise Refused(f"the CMS could not be reached for {tool}: "
                          f"{type(last).__name__}: {last}")
        return decode(tool, raw)


def decode(tool: str, raw: bytes) -> dict:
    """The tool's answer out of one JSON-RPC response body.

    Every shape the CMS does not answer is a Refused that says what came
    instead: a maintenance or proxy page is HTML with status 200, and a body
    that is JSON but not the object JSON-RPC defines would otherwise end the
    run in an AttributeError.
    """
    try:
        answer = json.loads(raw)
    except ValueError:  # not JSON, or bytes that are not text
        head = raw[:200].decode("utf-8", "replace")
        raise Refused(f"{tool}: the CMS answered something that is not JSON "
                      f"(a proxy or maintenance page?): {head}")
    if not isinstance(answer, dict):
        raise Refused(f"{tool}: the CMS answered JSON that is not an object")
    error = answer.get("error")
    if error:
        if not isinstance(error, dict):
            raise Refused(f"{tool}: the CMS answered an error it did not describe: {str(error)[:200]}")
        raise RpcError(tool, error.get("code"), error.get("message") or "error without a message")
    result = answer.get("result") or {}
    content = result.get("content") if isinstance(result, dict) else None
    if not isinstance(result, dict) or not isinstance(content or [], list):
        raise Refused(f"{tool}: the CMS answered a result of an unexpected shape")
    text = "".join(c["text"] for c in content or []
                   if isinstance(c, dict) and isinstance(c.get("text"), str))
    if result.get("isError"):
        raise ToolError(tool, text)
    try:
        payload = json.loads(text)
    except json.JSONDecodeError:
        raise Refused(f"{tool} answered something that is not JSON: {text[:200]}")
    if not isinstance(payload, dict):
        raise Refused(f"{tool} answered JSON that is not an object: {text[:200]}")
    return payload


# ------------------------------------------------------------------ helpers


def notice(msg: str):
    print(f"::notice::{msg}" if os.environ.get("GITHUB_ACTIONS") else f"notice: {msg}")


def warn(msg: str):
    print(f"::warning::{msg}" if os.environ.get("GITHUB_ACTIONS") else f"warning: {msg}")


def vkey(v: str) -> tuple:
    return tuple(int(x) for x in v.split("."))


def place(endpoint: str) -> str:
    """The CMS's name in a message: its host (holzcloud.ch by default)."""
    return urllib.parse.urlsplit(endpoint).netloc or endpoint


def find(listed: dict, key: str):
    """The entry of key in a list_values answer, or None."""
    values = listed.get("values") or []
    if not isinstance(values, list):
        raise Refused("list_values answered values that are not a list")
    for entry in values:
        if isinstance(entry, dict) and entry.get("key") == key:
            return entry
    return None


def text_of(entry) -> str | None:
    """An entry's text, or None when there is no entry or no text."""
    value = entry.get("value") if entry else None
    return None if value is None else str(value)


def public_path(page: dict) -> str:
    """A page's public address; "language" is only given outside the main language."""
    lang = page.get("language")
    return f"/{lang}/{page['slug']}" if lang else "/" + page["slug"]


# ------------------------------------------------------------------ the run


def run(args) -> int:
    project = ALIASES.get(args.project, args.project)
    key = PROJECTS[project]
    website = args.website
    where = place(args.endpoint)

    version = args.version.strip()
    version = version[1:] if version.startswith("v") else version
    if not PLAIN.fullmatch(version):
        notice(f"'{args.version}' is not a plain X.Y.Z release (a beta or a release "
               f"candidate); {where} keeps showing the last plain release.")
        return 0

    token = os.environ.get("CMS_TOKEN", "").strip()
    if not token:
        notice(f"CMS_TOKEN is not set, so {where} was not updated. Add the secret CMS_TOKEN "
               f"(a content key for website {website}) to update it on every release.")
        return 0

    cms = CMS(args.endpoint, token)
    try:
        listed = cms.call("list_values", {"website": website})
    except RpcError as e:
        if e.code != NO_SUCH_TOOL:
            raise
        notice(f"{where} runs a CMS without values (older than 0.1.5); nothing was set. "
               f"Once it runs 0.1.5, this sets {key} to the released version.")
        return 0

    entry = find(listed, key)
    current = text_of(entry)
    if current is None:
        print(f"{key} does not exist on website {website} yet; it is created with {version}.")
    elif not PLAIN.fullmatch(current):
        warn(f"{key} holds '{current}', which is not a plain X.Y.Z version; it is replaced with {version}.")
    elif vkey(current) > vkey(version) and not args.allow_older:
        notice(f"{where} already shows {current} ({key}), newer than {version}; nothing set "
               "(--allow-older overrides).")
        return 0
    elif vkey(current) == vkey(version):
        print(f"{where} already shows {version} ({key}); nothing to set.")
        return 0

    if args.dry_run:
        print(f"Dry run: would set {key} on website {website} from {current or 'nothing'} to {version}.")
        return 0

    answer = cms.call("set_value", {"website": website, "key": key, "value": version})
    print(f"set {key} on website {website}: {current or 'nothing'} -> {answer.get('value')}"
          + (" (created)" if answer.get("created") else ""))

    # Read back. A write the CMS accepted is not yet a value that says the
    # right thing; this is the check that it does.
    listed = cms.call("list_values", {"website": website})
    entry = find(listed, key)
    got = text_of(entry)
    if got != version:
        raise Refused(f"{key} reads {got or 'nothing'} after it was set to {version}")
    pages = entry.get("used_on") or []
    pages = [p for p in pages if isinstance(p, dict)] if isinstance(pages, list) else []
    print(f"Read back: {key} = {version} on website {website}, named by {len(pages)} page(s).")

    if args.site:
        check_public(args.site, key, pages, version)
    return 0


def check_public(site: str, key: str, pages: list, version: str):
    """What a visitor gets. A warning only: a cache may lag behind."""
    published = [p for p in pages if p.get("status") == "published" and isinstance(p.get("slug"), str)]
    if not published:
        notice(f"no published page names [[wert:{key}]] yet, so no page shows {version}.")
        return
    for page in published:
        url = site.rstrip("/") + public_path(page)
        try:
            req = urllib.request.Request(url, headers={"User-Agent": "holzcloud-cms-version/2"})
            with urllib.request.urlopen(req, timeout=30) as res:
                html = res.read().decode("utf-8", "replace")
        except Exception as e:  # noqa: BLE001 - reported, not fatal
            warn(f"{url} could not be fetched: {e}")
            continue
        if version in html:
            print(f"public {url}: shows {version}")
        else:
            warn(f"{url} does not show {version} yet")


def website_default() -> int:
    raw = os.environ.get("CMS_WEBSITE", "").strip()
    return int(raw) if raw else DEFAULT_WEBSITE


def main(argv=None) -> int:
    p = argparse.ArgumentParser(description=__doc__.split("\n\n")[0])
    p.add_argument("--project", required=True, choices=sorted(set(PROJECTS) | set(ALIASES)))
    p.add_argument("--version", required=True, help="the released tag, e.g. v0.2.3")
    p.add_argument("--dry-run", action="store_true", help="read and print the plan, set nothing")
    p.add_argument("--allow-older", action="store_true", help="set even if the value holds a newer version")
    p.add_argument("--website", type=int, default=None,
                   help=f"the website's number in the CMS (default: CMS_WEBSITE, else {DEFAULT_WEBSITE})")
    p.add_argument("--endpoint", default=os.environ.get("CMS_ENDPOINT") or DEFAULT_ENDPOINT,
                   help="the CMS's MCP address")
    p.add_argument("--site", default=os.environ.get("CMS_SITE", DEFAULT_SITE),
                   help="public base address to check after the write; empty skips it")
    args = p.parse_args(argv)
    if args.website is None:
        try:
            args.website = website_default()
        except ValueError:
            p.error("CMS_WEBSITE must be a number")
    try:
        return run(args)
    except Refused as e:
        error(f"{place(args.endpoint)} was not updated: {e}")
        return 1
    except Exception as e:  # noqa: BLE001 - a run ends in one line, never a traceback
        error(f"{place(args.endpoint)} was not updated: unexpected {type(e).__name__}: {e}")
        return 1


def error(msg: str):
    print(f"::error::{msg}" if os.environ.get("GITHUB_ACTIONS") else f"error: {msg}", file=sys.stderr)


if __name__ == "__main__":
    sys.exit(main())
