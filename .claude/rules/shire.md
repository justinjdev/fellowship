# Shire — codebase search index

Shire provides a pre-built search index (FTS5 + optional vector search) for this codebase.
It indexes packages, symbols, files, and the dependency graph.

## Default to Shire for search

Use Shire tools before falling back to Grep/Glob:

- **Find a function/class/type:** `search_symbols` — returns structured results with signature, file path, and line number
- **Find a file:** `search_files` — searches by path or name
- **Find a package:** `search_packages` — searches by name or description
- **Explore a concept:** `explore` — broad semantic search returning a structured context map
- **Understand a file:** `get_file_symbols` — list all symbols without reading the file
- **Understand a package's API:** `search_symbols` with a package filter — list all exported symbols

## Use Grep/Glob when

- Searching for literal strings, log messages, or error text
- Searching inside function bodies (Shire indexes definitions, not implementations)
- Pattern matching on file contents

## Before modifying shared code

- `package_dependents` — check what depends on the package you're changing
- `package_dependencies` with depth>1 — see the full transitive dependency chain
