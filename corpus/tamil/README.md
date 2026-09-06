# Tamil corpus

Example and (later) test programs for Niraluli. Subset: **Tamil-0 (frozen 2026-08-01)**.

Corpus programs are part of the frozen snapshot; treat changes as subset bumps unless fixing comments/docs only.

## Valid

| File | Covers |
|------|--------|
| `வணக்கம்.uli` | package, entry `தொடக்கம்`, `பதிப்பி` string |
| `எண்கணிதம்.uli` | `மாறி`, `:=`, `=`, arithmetic `+ - * / %` |
| `நிபந்தனை.uli` | `எனில்` / `இல்லையேல்`, `மெய்` / comparisons |
| `கூட்டு.uli` | helper `செயல்பாடு`, params, `திருப்பு`, call |
| `சுழல்.uli` | Tamil-0.1 | `சுழல்` C-style + while-like loops |
| `பல்கோப்பு/` | Tamil-0.9 | same-package multi-file (`uli பல்கோப்பு`) |
| `கொணர்_எடு/` | Tamil-0.10 | `கொணர் "கணிதம்"` + `கணிதம்.கூட்டு` |
| `வலை_எதிரொலி/` | Tamil-0.53 | TCP echo via `கொணர் "வலை"` |
| `வலைபரிமாற்றம்/` | Tamil-0.59 | query params + request headers + exact-path mux |
| `வலைபரிமாற்றம்_html/` | Tamil-0.54 | HTTP HTML response |
| `வலை_பிழை/` | Tamil-0.55 | TCP error result (`*வலை.பிழை`) |
| `வலை_வாடிக்கையாளர்/` | Tamil-0.59 | HTTP client + headers + chunked response |
| `வலை_வாடிக்கையாளர்_பிழை/` | Tamil-0.57 | HTTPS/URL/reserved-header errors |
| `வலை_திருப்பு/` | Tamil-0.61 | opt-in HTTP redirect + timeout options |
| `தகவல்_வலை/` | Tamil-0.61 | IPv4 UDP loopback datagrams |
| `தரவுத்தளம்/` | Tamil-0.62 | shared SQL vtable + SQLite prepared statements |
| `ஊழியர்_முழுஅடுக்கு/` | Tamil-0.62 | React employee grid + Niraluli REST API + Tamil SQLite (`SETUP.md`) |
| `கோப்பு/` | Tamil-0.63 | POSIX file open/read/write via `கொணர் "கோப்பு"` |
| `நேரம்/` | Tamil-0.64 | wall clock, sleep, duration helpers via `கொணர் "நேரம்"` |
| `வடிவமைப்பு/` | Tamil-0.66/0.77 | fmt `%s` and typed verbs via `கொணர் "வடிவம்"` |
| `பலவாதங்கள்.uli` | Tamil-0.66/0.69 | variadic `...முழுஎண்` and `slice...` expand |
| `மாறிலி.uli` | Tamil-0.67 | package + local compile-time constants |
| `மாறி_தொகுப்பு.uli` | Tamil-0.70 | package-level variables |
| `பொதுவகை_வகை.uli` | Tamil-0.71 | generic types and constraints |
| `பொதுவகை_முறை.uli` | Tamil-0.72 | generic methods on generic types |
| `மாறி_குழு.uli` | Tamil-0.73 | package/block `மாறி ( … )` groups |
| `இருமியியக்கம்.uli` | Tamil-0.74 | bitwise/shifts and iota flags |
| `முழுஅகலம்.uli` | Tamil-0.75 | integer widths |
| `பொதுவகை_மாற்று.uli` | Tamil-0.76 | generic type aliases |
| `வடிவமைப்பு/` | Tamil-0.66/0.77 | fmt `%s` and typed verbs |
| `செயல்பாடு_புலம்.uli` | Tamil-0.58 | function values in struct and slice fields |
| `குவியல்.uli` | Tamil-0.60 | indexed conservative GC + flattened string concat |

## Invalid (`invalid/`)

| File | Why invalid |
|------|-------------|
| `தொகுப்பு_இல்லை.uli` | missing `தொகுப்பு` clause |
| `அறிவிக்கப்படாத_ஒதுக்கீடு.uli` | `=` without prior `மாறி` / `:=` (syntax OK; semantic error later) |
| `பழைய_else.uli` | uses `இல்லையெனில்` (IDENT); not parse-fatal — style/semantic later |
| `முழுமையற்ற_ஒதுக்கீடு.uli` | `=` with missing expression (parse error) |

## Conventions

- One idea per file where practical.
- Basenames may be Tamil; extension is always `.uli`.
- ASCII fallback names are for docs only (not checked in).
- Invalid programs are expected to fail once the checker/parser exists.
- Comments use Go style (`//` line, `/* */` block). Each file includes a Go-equivalent sketch in a leading `//` comment.
- Tamil must render in the editor: see [`notes/fonts-tamil.md`](../../notes/fonts-tamil.md) (Noto Sans Tamil).

When adding a program, update this table and prefer matching a construct card under `grammar/tamil/constructs/`.
