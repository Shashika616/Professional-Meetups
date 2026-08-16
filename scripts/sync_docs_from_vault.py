#!/usr/bin/env python3
"""
Regenerate docs/ in this repo from the Obsidian vault ("Professional Meetups Vault").

The vault is the source of truth for product/architecture/decisions content.
docs/ is a generated, read-only mirror: Obsidian [[wikilinks]] get rewritten
into plain relative markdown links so the files render normally on GitHub, in
plain markdown previewers, and for Claude Code (which doesn't understand
Obsidian's wikilink syntax).

Usage:
    python3 scripts/sync_docs_from_vault.py [--vault PATH] [--docs PATH]

Run this after editing any note in the vault, before committing. It's a pure
mechanical find-and-replace over the vault's markdown files — no content is
regenerated or reworded, so it's cheap and safe to run as often as you like.
Do not hand-edit files under docs/; edit the vault and re-run this instead,
or your changes will be silently overwritten next sync.
"""
import argparse
import os
import re

DEFAULT_VAULT = os.path.expanduser("~/Documents/Professional Meetups Vault")
DEFAULT_DOCS = os.path.join(os.path.dirname(os.path.dirname(os.path.abspath(__file__))), "docs")

FOLDER_MAP = {
    "00 - Project": "00-project",
    "01 - Product": "01-product",
    "02 - Domain": "02-domain",
    "03 - Architecture": "03-architecture",
    "04 - Decisions": "04-decisions",
    "05 - UX": "05-ux",
    "06 - Roadmap": "06-roadmap",
    "07 - Research": "07-research",
}

WIKILINK_RE = re.compile(r"\[\[([^\]|#]+)(#[^\]|]+)?(\|[^\]]+)?\]\]")
FRONTMATTER_RE = re.compile(r"^---\n.*?\n---\n\n?", re.DOTALL)
ADR_BARE_RE = re.compile(r"(?<!\[)\bADR-(\d{3})\b(?!\]|\()")


def kebab(name: str) -> str:
    name = name.replace("&", "and")
    name = re.sub(r"[^A-Za-z0-9]+", "-", name).strip("-")
    return name.lower()


def build_mapping(vault: str):
    """basename (no .md) -> path relative to docs/ root, e.g. 'trust-levels.md'."""
    mapping = {}
    files = []
    for root, _dirs, fs in os.walk(vault):
        if ".obsidian" in root:
            continue
        for f in fs:
            if not f.endswith(".md"):
                continue
            full = os.path.join(root, f)
            basename = f[:-3]
            rel_folder = os.path.relpath(root, vault)
            files.append((full, basename, rel_folder))

    for _full, basename, rel_folder in files:
        if rel_folder == ".":
            new_rel = "README.md" if basename == "Welcome" else f"{kebab(basename)}.md"
        else:
            sub = FOLDER_MAP.get(rel_folder, kebab(rel_folder))
            new_rel = f"{sub}/{kebab(basename)}.md"
        mapping[basename] = new_rel
    return mapping, files


def convert(content: str, my_new_rel: str, mapping: dict, adr_map: dict) -> str:
    content = FRONTMATTER_RE.sub("", content, count=1)
    my_dir = os.path.dirname(my_new_rel)

    def repl_wikilink(m):
        target_basename = m.group(1).strip()
        target_rel = mapping.get(target_basename)
        if not target_rel:
            return f"**{target_basename}**"  # unresolved link -> bold text, not a dead link
        rel_from_here = os.path.relpath(target_rel, my_dir) if my_dir else target_rel
        return f"[{target_basename}]({rel_from_here})"

    content = WIKILINK_RE.sub(repl_wikilink, content)

    def repl_adr_bare(m):
        num = m.group(1)
        target_basename = adr_map.get(num)
        if not target_basename:
            return m.group(0)
        target_rel = mapping[target_basename]
        rel_from_here = os.path.relpath(target_rel, my_dir) if my_dir else target_rel
        return f"[ADR-{num}]({rel_from_here})"

    return ADR_BARE_RE.sub(repl_adr_bare, content)


def main():
    parser = argparse.ArgumentParser(description=__doc__)
    parser.add_argument("--vault", default=DEFAULT_VAULT, help="Path to the Obsidian vault")
    parser.add_argument("--docs", default=DEFAULT_DOCS, help="Path to this repo's docs/ folder")
    args = parser.parse_args()

    if not os.path.isdir(args.vault):
        raise SystemExit(f"Vault not found at {args.vault!r} — pass --vault PATH")

    mapping, files = build_mapping(args.vault)
    adr_map = {}
    for basename in mapping:
        m = re.match(r"ADR-(\d{3})", basename)
        if m:
            adr_map[m.group(1)] = basename

    os.makedirs(args.docs, exist_ok=True)
    written = []
    for full, basename, _rel_folder in files:
        new_rel = mapping[basename]
        with open(full, encoding="utf-8") as fh:
            raw = fh.read()
        converted = convert(raw, new_rel, mapping, adr_map)
        out_path = os.path.join(args.docs, new_rel)
        os.makedirs(os.path.dirname(out_path) or args.docs, exist_ok=True)
        with open(out_path, "w", encoding="utf-8") as fh:
            fh.write(converted)
        written.append(out_path)

    print(f"Synced {len(written)} files from vault -> {args.docs}")


if __name__ == "__main__":
    main()
