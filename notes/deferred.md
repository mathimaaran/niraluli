# Deferred capabilities

Natural enhancements kept out of the current subset so each Tamil-0.x stay small and closed-loop.
**Ordered next work** stays in [`subset-roadmap.md`](subset-roadmap.md); this file is the inventory of *what* we still want, with sources.

Update when a subset ships (strike or move items) or when a construct card records a new cut.

## Likely next (NASM next; leftovers stay backlog)

| Item | Notes | Source |
|------|--------|--------|
| NASM x86-64 → i386 | Next major backend work | roadmap |
| Interfaces | Only if earned (e.g. Go-style `Handler`) | method.yaml |
| Generic types / constraints | After interfaces earn a place | generics.yaml |

Shipped: 0.11–0.66 (through variadic parameters `...T`).

## Backlog leftovers (pre-NASM optional; keep out of critical path)

| Item | Notes | Source |
|------|--------|--------|
| Dial / DNS timeouts | HTTP `நேரமுடிவு` currently covers send/receive sockets only | http.yaml |
| Relative HTTP redirects | Absolute `http://` only today | http.yaml |
| HTTPS / TLS | Explicitly out of scope for 0.61 | http.yaml / net.yaml |
| PostgreSQL / MySQL backends | Implement the Tamil-0.62 internal DB vtable | database.yaml |
| Database pooling | Connection handles are single connections today | database.yaml |
| Public database driver ABI | Internal C vtable is not a third-party plugin API | database.yaml |
| File seek / directories / chmod | Tamil-0.63 is open/read/write/close + whole-file helpers | file.yaml |
| Timers / tickers / parse / locations | Tamil-0.64 is now/sleep/since + RFC3339 UTC format | time.yaml |
| Variadic / typed fmt verbs (`%d`) | Tamil-0.66 has `...T`; `%d`/`any` still later | fmt.yaml |
| `slice...` expand at call sites | Tamil-0.66 packs trailing args only | variadic.yaml |
| Interfaces | Only if earned | method.yaml |

## Struct & types

| Item | Notes | Source |
|------|--------|--------|
| ~~string ↔ []byte / []rune~~ | Tamil-0.39 (`இருமி8` / `இருமி32`) | byte-rune.yaml |

## Control & functions

| Item | Notes | Source |
|------|--------|--------|
| ~~bare zero-arg defer~~ | Tamil-0.42 | defer.yaml |
| ~~defer method bind (`&x`)~~ | Tamil-0.43 | defer.yaml |
| ~~first-class method / function values~~ | Tamil-0.44 | func-value.yaml |
| ~~function literals / closures~~ | Tamil-0.45 | closure.yaml |
| ~~Go 1.22+ per-iteration loop vars~~ | Tamil-0.46 | closure.yaml |
| ~~method expressions `T.M` / `(*T).M`~~ | Tamil-0.47 | method-expr.yaml |
| ~~goroutines / channels / select~~ | Tamil-0.48 | goroutine.yaml |
| ~~panic / recover~~ | Tamil-0.49 (`அலறு` / `மீள்`) on C | panic.yaml |
| ~~panic / concurrency polish~~ | Tamil-0.50 primitives, traces, unbuffered select | panic.yaml / goroutine.yaml |
| ~~range over channels~~ | Tamil-0.51 `ஒவ்வொரு` over `தடம்` | range.yaml / goroutine.yaml |

## Packages, runtime, backends

| Item | Notes | Source |
|------|--------|--------|
| ~~GC / native heap~~ | Tamil-0.52 STW conservative mark-sweep on C | gc.yaml |
| ~~TCP sockets~~ | Tamil-0.53 `கொணர் "வலை"` (listen/dial/read/write) | net.yaml |
| ~~HTTP server~~ | Tamil-0.54 `கொணர் "பரிமாற்றம்"` (handler func values) | http.yaml |
| ~~Net / HTTP error returns~~ | Tamil-0.55 `*வலை.பிழை`; EOF remains `(0, இன்மை)` | net.yaml / http.yaml |
| ~~HTTP request headers + mux~~ | Tamil-0.56 lowercase header map + exact paths | http.yaml |
| ~~HTTP client + response headers~~ | Tamil-0.57 `பெறு`/`பதிவிடு`/`கோரு` + `எழுதுதலைப்புகள்` | http.yaml |
| ~~function values in struct/slice fields~~ | Tamil-0.58 | func-value.yaml |
| ~~query parameters + chunked HTTP responses~~ | Tamil-0.59 | http.yaml |
| ~~GC/string polish + optimized C default~~ | Tamil-0.60 | gc.yaml / string.yaml |
| ~~HTTP timeout/redirect options + IPv4 UDP~~ | Tamil-0.61 | http.yaml / net.yaml |
| ~~Shared SQL vtable + SQLite backend~~ | Tamil-0.62 `கொணர் "தரவுத்தளம்"` | database.yaml |
| ~~POSIX file I/O~~ | Tamil-0.63 `கொணர் "கோப்பு"` | file.yaml |
| ~~Time / duration~~ | Tamil-0.64 `கொணர் "நேரம்"` | time.yaml |
| ~~fmt-style formatting~~ | Tamil-0.65 `கொணர் "வடிவம்"` | fmt.yaml |
| ~~Variadic parameters~~ | Tamil-0.66 `...T` | variadic.yaml |
| Goroutines / channels | Tamil-0.48 on C (pthread); see goroutine.yaml | goroutine.yaml |
| NASM + custom IR | IR when NASM begins | design-decisions |
| Windows / macOS primary targets | Linux-first | Non-goals |

## Larger Go-inspired (only if earned)

Interfaces, richer generics (types/constraints), concurrency, full stdlib parity, Go source compatibility, self-hosting, phonetic Tamil keywords.

See **Non-goals** in [`design-decisions.md`](design-decisions.md).

## How to promote an item

1. Propose the subset in `design-decisions.md` (dated).
2. Lock keywords / EBNF / construct card.
3. Add corpus, then lex → parse → check → emit → run.
4. Check the item off here and on `subset-roadmap.md`.
