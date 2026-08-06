#!/usr/bin/env python3
"""Regenerate docs/askrelay-overview.html — the maintainer's single-page reading
copy of every project document.

Self-contained output: all markdown is embedded, rendered client-side by a
vendored copy of marked.js (docs/tools/marked.min.js, MIT). No network, no
external files — the HTML works from file:// forever.

Run after editing any markdown doc (or `make overview`), commit the result.
Stdlib only; no pip dependencies.
"""

import json
import subprocess
import sys
from datetime import datetime, timezone
from pathlib import Path

ROOT = Path(__file__).resolve().parents[2]
OUT = ROOT / "docs" / "askrelay-overview.html"
MARKED = Path(__file__).resolve().parent / "marked.min.js"

# (key, tab title, path). Order = sidebar order. Default tab = first entry.
DOCS = [
    ("plan", "Implementation plan", "docs/askrelay-implementation-plan.md"),
    ("arch", "Architecture", "docs/askrelay-architecture.md"),
    ("sysmap", "System map", "docs/askrelay-system-map.md"),
    ("launch", "Launch checklist", "docs/askrelay-launch-checklist.md"),
    ("decisions", "Decision log", "docs/phase2-decision-log.md"),
    ("readme", "README", "README.md"),
    ("claude", "CLAUDE.md", "CLAUDE.md"),
]


def git(*args: str) -> str:
    try:
        return subprocess.run(
            ["git", *args], cwd=ROOT, capture_output=True, text=True, check=True
        ).stdout.strip()
    except Exception:
        return "?"


def main() -> int:
    docs = []
    for key, title, rel in DOCS:
        p = ROOT / rel
        if not p.exists():
            print(f"warning: {rel} missing, skipped", file=sys.stderr)
            continue
        docs.append({"key": key, "title": title, "file": rel,
                     "md": p.read_text(encoding="utf-8")})

    payload = json.dumps(docs, ensure_ascii=False).replace("</", "<\\/")
    marked_js = MARKED.read_text(encoding="utf-8")
    generated = datetime.now(timezone.utc).strftime("%Y-%m-%d %H:%M UTC")
    commit = git("rev-parse", "--short", "HEAD")

    html = (
        TEMPLATE
        .replace("__DOCS_JSON__", payload)
        .replace("__MARKED_JS__", marked_js)
        .replace("__GENERATED__", generated)
        .replace("__COMMIT__", commit)
    )
    OUT.write_text(html, encoding="utf-8")
    print(f"wrote {OUT.relative_to(ROOT)} ({OUT.stat().st_size // 1024} KB, "
          f"{len(docs)} docs, commit {commit})")
    return 0


TEMPLATE = r"""<!DOCTYPE html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>askrelay — project overview</title>
<style>
  :root {
    --bg: #ffffff; --fg: #1f2328; --muted: #656d76; --border: #d1d9e0;
    --accent: #0969da; --code-bg: #f6f8fa; --sidebar-bg: #f6f8fa;
    --mark: #fff8c5;
  }
  @media (prefers-color-scheme: dark) { :root:not([data-theme="light"]) {
    --bg: #0d1117; --fg: #e6edf3; --muted: #8b949e; --border: #30363d;
    --accent: #4493f8; --code-bg: #161b22; --sidebar-bg: #10151c;
    --mark: #3a3000;
  } }
  :root[data-theme="dark"] {
    --bg: #0d1117; --fg: #e6edf3; --muted: #8b949e; --border: #30363d;
    --accent: #4493f8; --code-bg: #161b22; --sidebar-bg: #10151c;
    --mark: #3a3000;
  }
  * { box-sizing: border-box; }
  html { scroll-behavior: smooth; }
  body {
    margin: 0; background: var(--bg); color: var(--fg);
    font: 16px/1.6 -apple-system, BlinkMacSystemFont, "Segoe UI", Helvetica, Arial, sans-serif;
    display: flex; min-height: 100vh;
  }
  nav {
    width: 270px; flex: 0 0 270px; background: var(--sidebar-bg);
    border-right: 1px solid var(--border); padding: 1rem;
    position: sticky; top: 0; height: 100vh; overflow-y: auto; font-size: 14px;
  }
  nav .brand { font-weight: 700; font-size: 17px; margin-bottom: .1rem; }
  nav .meta { color: var(--muted); font-size: 12px; margin-bottom: 1rem; }
  nav .tabs { display: flex; flex-direction: column; gap: 2px; margin-bottom: 1rem; }
  nav .tabs button {
    text-align: left; padding: .45rem .6rem; border: 0; border-radius: 6px;
    background: transparent; color: var(--fg); font: inherit; cursor: pointer;
  }
  nav .tabs button:hover { background: color-mix(in srgb, var(--accent) 12%, transparent); }
  nav .tabs button.active { background: var(--accent); color: #fff; font-weight: 600; }
  nav .toc { border-top: 1px solid var(--border); padding-top: .8rem; }
  nav .toc a {
    display: block; color: var(--muted); text-decoration: none;
    padding: .15rem 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap;
  }
  nav .toc a:hover { color: var(--accent); }
  nav .toc a.h3 { padding-left: 1rem; font-size: 13px; }
  nav .theme { margin-top: 1rem; }
  nav .theme button {
    font: inherit; font-size: 12px; padding: .3rem .6rem; border-radius: 6px;
    border: 1px solid var(--border); background: transparent; color: var(--muted); cursor: pointer;
  }
  main { flex: 1; min-width: 0; padding: 2rem 3rem 6rem; }
  article { max-width: 54rem; margin: 0 auto; display: none; }
  article.active { display: block; }
  article .srcpath { color: var(--muted); font-size: 12px; border-bottom: 1px solid var(--border); padding-bottom: .5rem; }
  h1, h2, h3, h4 { line-height: 1.25; scroll-margin-top: 1rem; }
  h1 { font-size: 1.9em; border-bottom: 1px solid var(--border); padding-bottom: .3em; }
  h2 { font-size: 1.45em; border-bottom: 1px solid var(--border); padding-bottom: .3em; margin-top: 2em; }
  h3 { font-size: 1.15em; margin-top: 1.6em; }
  a { color: var(--accent); }
  code, pre {
    font-family: ui-monospace, SFMono-Regular, Menlo, Consolas, monospace; font-size: 85%;
  }
  code { background: var(--code-bg); padding: .15em .35em; border-radius: 4px; }
  pre { background: var(--code-bg); padding: 1rem; border-radius: 8px; overflow-x: auto; line-height: 1.45; }
  pre code { background: transparent; padding: 0; }
  blockquote {
    margin: 1em 0; padding: .2em 1em; color: var(--muted);
    border-left: 4px solid var(--border);
  }
  .tblwrap { overflow-x: auto; margin: 1em 0; }
  table { border-collapse: collapse; font-size: 14px; }
  th, td { border: 1px solid var(--border); padding: .4em .7em; text-align: left; vertical-align: top; }
  th { background: var(--code-bg); }
  tr:nth-child(even) td { background: color-mix(in srgb, var(--code-bg) 55%, transparent); }
  hr { border: 0; border-top: 1px solid var(--border); margin: 2.5em 0; }
  li { margin: .15em 0; }
  input[type="checkbox"] { margin-right: .4em; }
  mark { background: var(--mark); color: inherit; }
  #menu-btn { display: none; }
  @media (max-width: 900px) {
    nav { position: fixed; z-index: 10; transform: translateX(-100%); transition: transform .2s; }
    nav.open { transform: none; }
    main { padding: 1rem 1.2rem 4rem; }
    #menu-btn {
      display: block; position: fixed; top: .7rem; right: .7rem; z-index: 11;
      padding: .4rem .7rem; border-radius: 8px; border: 1px solid var(--border);
      background: var(--sidebar-bg); color: var(--fg); font: inherit;
    }
  }
  @media print { nav, #menu-btn { display: none; } article { display: block !important; } }
</style>
</head>
<body>
<button id="menu-btn">☰ docs</button>
<nav id="nav">
  <div class="brand">askrelay — overview</div>
  <div class="meta">generated __GENERATED__ · commit <code>__COMMIT__</code><br>
  regenerate: <code>make overview</code></div>
  <div class="tabs" id="tabs"></div>
  <div class="toc" id="toc"></div>
  <div class="theme"><button id="theme-btn">toggle light/dark</button></div>
</nav>
<main id="main"></main>

<script>__MARKED_JS__</script>
<script type="application/json" id="docs-data">__DOCS_JSON__</script>
<script>
(function () {
  const DOCS = JSON.parse(document.getElementById('docs-data').textContent);
  const byFile = {};   // "askrelay-architecture.md" -> key
  DOCS.forEach(d => { byFile[d.file.split('/').pop().toLowerCase()] = d.key; });

  // --- GitHub-compatible heading slugs (must match the anchors used in the docs)
  function slugger() {
    const used = new Set();
    return function (text) {
      let s = text.toLowerCase().trim()
        .replace(/[^\p{L}\p{N}\s_-]/gu, '')
        .replace(/\s/g, '-');
      let base = s, i = 1;
      while (used.has(s)) s = base + '-' + (i++);
      used.add(s);
      return s;
    };
  }

  marked.use({ gfm: true, breaks: false, mangle: false });

  const main = document.getElementById('main');
  const tabs = document.getElementById('tabs');
  const toc = document.getElementById('toc');
  const tocByKey = {};

  DOCS.forEach(doc => {
    const art = document.createElement('article');
    art.id = 'doc-' + doc.key;
    art.innerHTML = '<div class="srcpath">' + doc.file + '</div>' + marked.parse(doc.md);
    main.appendChild(art);

    // heading ids, prefixed per doc so slugs never collide across docs
    const slug = slugger();
    const heads = art.querySelectorAll('h1, h2, h3, h4, h5');
    heads.forEach(h => { h.id = doc.key + '--' + slug(h.textContent); });

    // wrap tables for horizontal scroll
    art.querySelectorAll('table').forEach(t => {
      const w = document.createElement('div'); w.className = 'tblwrap';
      t.parentNode.insertBefore(w, t); w.appendChild(t);
    });

    // rewrite links: in-doc anchors, cross-doc .md links, external targets
    art.querySelectorAll('a[href]').forEach(a => {
      const href = a.getAttribute('href');
      if (href.startsWith('#')) {
        a.setAttribute('href', '#' + doc.key + '--' + href.slice(1));
        a.dataset.doc = doc.key;
      } else if (/\.md(#|$)/i.test(href)) {
        const m = href.match(/([^\/]+\.md)(?:#(.*))?$/i);
        const target = m && byFile[m[1].toLowerCase()];
        if (target) {
          a.setAttribute('href', m[2] ? '#' + target + '--' + m[2] : '#doc-' + target);
          a.dataset.doc = target;
        }
      } else if (/^https?:/i.test(href)) {
        a.target = '_blank'; a.rel = 'noopener';
      }
    });

    // per-doc TOC (h2 + h3)
    const frag = document.createDocumentFragment();
    art.querySelectorAll('h2, h3').forEach(h => {
      const a = document.createElement('a');
      a.href = '#' + h.id; a.dataset.doc = doc.key;
      a.textContent = h.textContent;
      if (h.tagName === 'H3') a.className = 'h3';
      frag.appendChild(a);
    });
    tocByKey[doc.key] = frag;

    const btn = document.createElement('button');
    btn.textContent = doc.title; btn.dataset.doc = doc.key;
    btn.addEventListener('click', () => { activate(doc.key); window.scrollTo(0, 0); });
    tabs.appendChild(btn);
  });

  function activate(key) {
    document.querySelectorAll('article').forEach(a => a.classList.toggle('active', a.id === 'doc-' + key));
    tabs.querySelectorAll('button').forEach(b => b.classList.toggle('active', b.dataset.doc === key));
    toc.replaceChildren(tocByKey[key].cloneNode(true));
    document.getElementById('nav').classList.remove('open');
  }

  // any click on a doc-targeting link activates the right tab first
  document.addEventListener('click', e => {
    const a = e.target.closest('a[data-doc]');
    if (!a) return;
    activate(a.dataset.doc);
    const id = a.getAttribute('href').slice(1);
    const el = document.getElementById(id);
    if (el) { e.preventDefault(); el.scrollIntoView(); history.replaceState(null, '', '#' + id); }
  });

  // deep link support: #dockey--heading or #doc-dockey
  function fromHash() {
    const h = location.hash.slice(1);
    if (!h) { activate(DOCS[0].key); return; }
    const key = h.startsWith('doc-') ? h.slice(4) : h.split('--')[0];
    activate(DOCS.some(d => d.key === key) ? key : DOCS[0].key);
    const el = document.getElementById(h);
    if (el) setTimeout(() => el.scrollIntoView(), 0);
  }
  fromHash();

  document.getElementById('menu-btn').addEventListener('click',
    () => document.getElementById('nav').classList.toggle('open'));
  document.getElementById('theme-btn').addEventListener('click', () => {
    const r = document.documentElement;
    const dark = (r.dataset.theme || (matchMedia('(prefers-color-scheme: dark)').matches ? 'dark' : 'light')) === 'dark';
    r.dataset.theme = dark ? 'light' : 'dark';
    try { localStorage.setItem('askrelay-theme', r.dataset.theme); } catch (e) {}
  });
  try {
    const saved = localStorage.getItem('askrelay-theme');
    if (saved) document.documentElement.dataset.theme = saved;
  } catch (e) {}
})();
</script>
</body>
</html>
"""

if __name__ == "__main__":
    sys.exit(main())
