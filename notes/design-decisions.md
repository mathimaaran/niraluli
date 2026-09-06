# Niraluli — design decisions

Locked decisions for Phase 0. Change only with an explicit note and date.

## Identity

| Decision | Choice | Locked |
|----------|--------|--------|
| Language name | **Niraluli** (நிரலுளி) — virtue / rightness | 2026-07-25 |
| Source extension | `.uli` (Latin). Tamil may appear in basenames. | 2026-07-25 |
| Hello program | `வணக்கம்.uli` (ASCII fallback: `vanakkam.uli`) | 2026-07-25 |
| Keyword style | **Semantic Tamil** (meaning, not English phonetics) | 2026-07-25 |
| Relation to Go | **Go-inspired, free to diverge** | 2026-07-25 |
| Host language (compiler) | **Go** | 2026-07-25 |

## Architecture (frontend-first)

| Decision | Choice | Locked |
|----------|--------|--------|
| Product focus | Tamil grammar / UX for end users | 2026-07-25 |
| Early backend | Emit **C**, compile with `gcc`/`clang` | 2026-07-25 |
| Later backend | Emit **NASM**, assemble/link for Linux | 2026-07-25 |
| First CPU target | x86-64 Linux (SysV); i386 later | 2026-07-25 |
| Parser approach | Hand-written recursive descent (learning + control) | 2026-07-25 |
| IR | Small custom IR when NASM backend begins; not required for C backend | 2026-07-25 |

## Language surface (defaults until revised)

| Decision | Choice |
|----------|--------|
| Digits in source | Arabic `0`–`9` primary; Tamil `௦`–`௯` OK in numeric literals (0.31) |
| Identifiers | Tamil and/or Latin letters; NFC normalization planned |
| Strings | UTF-8; Tamil string literals allowed |
| Operators / braces | Keep ASCII operators and `{}()[]` for Phase 0–1 |
| Semicolons | Go-like automatic semicolon insertion (lexer; Tamil-0.7) |
| Goroutines / channels | Tamil-0.48 on C (pthread) |
| GC | Tamil-0.52 STW conservative mark-sweep on C (native heap) |

## Non-goals (for now)

- Go source compatibility
- Full Go standard library parity
- Self-hosting
- Windows / macOS as primary targets
- Phonetic Tamil keywords

## Naming package

```text
Full name:  Niraluli (நிரலுளி)
Binary:     uli
Extension:  .uli
Hello:      வணக்கம்.uli
```

## Tamil-0.1 (2026-08-01)

Adds Go-style loops with keyword **சுழல்**, **முறி** (`break`), and **தொடர்** (`continue`).
See `grammar/tamil/constructs/for.yaml`, `break.yaml`, `continue.yaml`, and `corpus/tamil/சுழல்.uli`.

## Tamil-0.2 (2026-08-01)

Adds first-class strings: type keyword **சரம்**, variables/params/returns, `+` concatenation,
and `==` / `!=` content comparison. C backend uses `const char *`, `strcmp`, and a small
`uli_concat` helper (allocated; no free in Tamil-0.2).
See `grammar/tamil/constructs/string.yaml` and `corpus/tamil/சரம்.uli`.

## Tamil-0.3 (2026-08-01)

Adds Go-like slices: type `[]T` (T = முழுஎண் | நிலை | சரம்), composite literals `[]T{…}`,
index get/set, and builtin **நீளம்** (`len`). C backend uses fat pointers; out-of-bounds aborts.
No `ஒவ்வொரு`, append, or `xs[i:j]` yet.
See `grammar/tamil/constructs/slice.yaml` and `corpus/tamil/பட்டியல்.uli`.

## Tamil-0.4 (2026-08-01)

Adds **ஒவ்வொரு** (range) over slices inside `சுழல்`, Go-style:
`சுழல் i, v := ஒவ்வொரு xs { … }`, index-only, and blank `_`.
See `grammar/tamil/constructs/range.yaml` and `corpus/tamil/ஒவ்வொரு.uli`.

## Tamil-0.5 (2026-08-01)

- **சேர்** — append one element to a slice (`append`)
- **SliceExpr** — `xs[i:j]` / `xs[i:]` / `xs[:j]` / `xs[:]` on slices and `சரம்`
- **ஒவ்வொரு** over `சரம்` — byte index + rune (`முழுஎண்`), like Go
- **நீளம்** on `சரம்` — byte length
- **String/slice arena** — concat and growth allocate from an arena freed at end of `main`

See `grammar/tamil/constructs/append.yaml`, `slice-expr.yaml`, and `corpus/tamil/சேர்.uli`.

## Tamil-0.6 (2026-08-01)

Named structs: **வகை** (`type`) and **அமைப்பு** (`struct`), keyed literals, field `.` select/assign.
Field types in 0.6: முழுஎண், நிலை, சரம் only. No pointers or methods yet.
See `grammar/tamil/constructs/struct.yaml` and `corpus/tamil/அமைப்பு.uli`.

## Tamil-0.7 (2026-08-01)

Go-like pointers: `*T`, `&x`, `*p`; auto-deref field select on `*அமைப்பு`.
`T` may be முழுஎண், நிலை, சரம், named struct, or another pointer.
Struct field types stay scalars (no `*T` fields yet). No methods, no `nil` keyword.
See `grammar/tamil/constructs/pointer.yaml` and `corpus/tamil/சுட்டி.uli`.

## Tamil-0.8 (2026-08-01)

Methods: `செயல்பாடு (பெயர் T|*T) பெயர்(…)`, calls `X.பெயர்(…)`.
Named struct receivers only; Go-like addressability for pointer receivers.
See `grammar/tamil/constructs/method.yaml` and `corpus/tamil/முறை.uli`.

## Tamil-0.9 (2026-08-01)

Same-package multi-file: several `.uli` files with `தொகுப்பு தொடக்கம்`
share types/funcs/methods. CLI: `uli <dir>` or multiple paths.
Cross-package import keyword reserved: **கொணர்** (“bring”; verb — not noun இறக்குமதி).
See `grammar/tamil/constructs/multifile.yaml` and `corpus/tamil/பல்கோப்பு/`.

## Tamil-0.10 (2026-08-01)

**கொணர்** `"rel/path"` imports another package directory. Qualifier is the imported
`தொகுப்பு` name (`கணிதம்.கூட்டு`). All top-level names are visible (no privacy yet).
See `grammar/tamil/constructs/import.yaml` and `corpus/tamil/கொணர்_எடு/`.

## Tamil-0.11 (2026-08-01)

Opt-in export with **`வெளி`** (outside). Default package-private; same package sees
all names. Importers only see `வெளி` funcs/types/fields/methods. Replaces Go’s
capitalization rule (Tamil has no case). See `grammar/tamil/constructs/export.yaml`.

## Tamil-0.12 (2026-08-01)

Nil keyword **`இன்மை`** (absence / non-existence) plus pointer `==` / `!=`.
Untyped until used in a `*T` context; emits as C `NULL`. See
`grammar/tamil/constructs/nil.yaml`.

## Tamil-0.13 (2026-08-01)

Richer struct fields: nested named structs, `*T` (incl. recursive), and
`[]T` element scalars. Value-recursive structs rejected. See
`grammar/tamil/constructs/struct-fields.yaml`.

## Tamil-0.14 (2026-08-01)

Switch: **`திசைவி`** (director / router) + **`மற்றபடி`** (default). Case
clauses reuse **`எனில்`** (no separate case keyword). No fallthrough.
See `grammar/tamil/constructs/switch.yaml`.

## Tamil-0.15 (2026-08-01)

Go-style init before **`எனில்`** / **`திசைவி`**: `எனில் x := …; cond { … }`.
Scoped across body, cases, and `இல்லையேல்`. See `if-init.yaml`.

## Tamil-0.16 (2026-08-01)

Struct **`==` / `!=`**: field-wise, Go comparable rule (no slice fields).
See `grammar/tamil/constructs/struct-eq.yaml`.

## Tamil-0.17 (2026-08-01)

**`பதிப்பி`** of named structs: debug format `Type{field: value, …}`.
See `grammar/tamil/constructs/print-struct.yaml`.

## Tamil-0.18 (2026-08-01)

**`கொணர்` aliases**: `கொணர் க "கணிதம்"` then `க.கூட்டு`. Blank `_`
supported; no dot-import. See `grammar/tamil/constructs/import-alias.yaml`.

## Tamil-0.19 (2026-08-01)

Multi-**`சேர்`** and three-index slices **`xs[i:j:k]`** (Go rules). See
`grammar/tamil/constructs/slice-grow.yaml`.

## Tamil-0.20 (2026-08-01)

Positional struct literals `T{v1, v2, …}` (Go rules: no mix with keyed;
exact field count). See `grammar/tamil/constructs/struct-lit-pos.yaml`.

## Tamil-0.21 (2026-08-01)

Nested slices `[][]T` (and deeper) for leaf element types. See
`grammar/tamil/constructs/nested-slice.yaml`.

## Tamil-0.22 (2026-08-01)

**`ஆக்கு`** (make), **`நகல்`** (copy), **`திறன்`** (cap) for slices. See
`grammar/tamil/constructs/make-copy.yaml`.

## Tamil-0.23 (2026-08-01)

Multiple return values `(T, U)` with unpack via `:=` / `=`. See
`grammar/tamil/constructs/multi-return.yaml`.

## Tamil-0.24 (2026-08-01)

Fixed arrays `[N]T` (value types; sliceable to `[]T`). See
`grammar/tamil/constructs/array.yaml`.

## Tamil-0.25 (2026-08-01)

Slices of structs and pointers: `[]T`, `[]*T`. See
`grammar/tamil/constructs/slice-struct.yaml`.

## Tamil-0.26 (2026-08-01)

Named results `(ஈ முழுஎண், மீ முழுஎண்)`. See
`grammar/tamil/constructs/named-results.yaml`.

## Tamil-0.27 (2026-08-01)

Naked `திருப்பு` returns named result locals. See
`grammar/tamil/constructs/naked-return.yaml`.

## Tamil-0.28 (2026-08-01)

Parallel assign / swap `அ, ஆ = ஆ, அ`. See
`grammar/tamil/constructs/parallel-assign.yaml`.

## Tamil-0.29 (2026-08-01)

`வகை T = U` aliases and `வகை T U` defined types. See
`grammar/tamil/constructs/type-alias.yaml`.

## Tamil-0.30 (2026-08-01)

Float type keyword **மிதவைஎண்** (C `double`). See
`grammar/tamil/constructs/float.yaml`.

## Tamil-0.31 (2026-08-01)

Tamil digits `௦`–`௯` allowed in numeric literals (idents still Arabic). See
`grammar/tamil/constructs/tamil-digits.yaml`.

## Tamil-0.32 (2026-08-01)

Unused non-blank imports are errors; import cycles print `A → B → A`. See
`grammar/tamil/constructs/import-unused.yaml`.

## Tamil-0.33 (2026-08-01)

Conversions `T(x)`: defined ↔ underlying, `முழுஎண்` ↔ `மிதவைஎண்`. See
`grammar/tamil/constructs/conversion.yaml`.

## Tamil-0.34 (2026-08-01)

Bool/string/pointer conversions; mixed typed int/float arithmetic promotes to
`மிதவைஎண்`. See `grammar/tamil/constructs/conversion-extra.yaml`.

## Tamil-0.35 (2026-08-01)

Maps: type keyword **அகராதி**, builtin **நீக்கு**. Keys: `முழுஎண்`, `நிலை`,
`சரம்` (and defined types with those underlyings). Nil maps compare only to
`இன்மை`; assignment into a nil map aborts. C backend uses arena open-addressing
tables (handle is a pointer). See `grammar/tamil/constructs/map.yaml`.

## Tamil-0.36 (2026-08-01)

Map literals `அகராதி[K]V{k: v}` (keyed elements only; `{}` is non-nil empty).
`KeyValueExpr.Key` widened to any expression (struct field keys remain idents).
See `grammar/tamil/constructs/map-literal.yaml`.

## Tamil-0.37 (2026-08-01)

Defer keyword **தள்ளிவை** (“put aside”). CallExpr only; args evaluated now,
call runs LIFO at function return. C: arena-backed thunk stack + epilogue.
See `grammar/tamil/constructs/defer.yaml`.

## Tamil-0.38 (2026-08-01)

Map keys: any comparable type (floats, pointers, comparable structs/arrays).
See `grammar/tamil/constructs/map-keys.yaml`.

## Tamil-0.39 (2026-08-01)

`இருமி8` (byte/uint8) and `இருமி32` (rune/int32), plus `சரம்` ↔
`[]இருமி8` / `[]இருமி32`. Naming: இருமி = binary unit + bit width (not பைட்
or எழுத்து). See `grammar/tamil/constructs/byte-rune.yaml`.

## Tamil-0.40 (2026-08-01)

Function type parameters (unconstrained): `செயல்பாடு அடையாளம்[யா](x யா) யா`.
Conventional param name **யா**; no `any` keyword. Inference at calls;
optional explicit `அடையாளம்[முழுஎண்](x)`. C: monomorphize. Interfaces /
constraints / generic types deferred. See `constructs/generics.yaml`.

## Tamil-0.41 (2026-08-01)

Generics polish: multiple type params, `[]யா`, `f[T,U](…)`, exported
generics across `கொணர்`. See `constructs/generics.yaml`.

## Tamil-0.42 (2026-08-01)

Bare zero-arg defer sugar: `தள்ளிவை முடி` ≡ `தள்ளிவை முடி()`.
Conversions still rejected. See `constructs/defer.yaml`.

## Tamil-0.43 (2026-08-01)

Defer method bind: value receivers copied at `தள்ளிவை`; pointer
receivers on addressable values save `&x` (mutations before return are
visible). Applies to bare and `p.m()` forms. See `defer.yaml`.

## Tamil-0.44 (2026-08-01)

Function types (`செயல்பாடு(…) …` in type position) and first-class
method values / package function values. Receiver bind for method
values matches 0.43 defer. No function literals or interfaces in 0.44;
method expressions arrived in 0.47. See `constructs/func-value.yaml`.

## Tamil-0.45 (2026-08-01)

Function literals / closures: `செயல்பாடு(params) [results] { body }` as
expressions. Free variables captured by reference (Go-like); C arena
promotes captured locals. Builds on 0.44 `uli_fn`. See
`constructs/closure.yaml`.

## Tamil-0.46 (2026-08-01)

Go 1.22+ per-iteration loop variables: `சுழல்` / `ஒவ்வொரு` vars declared
with `:=` are distinct each iteration, so closures capture the correct
iteration values. See `constructs/closure.yaml`.

## Tamil-0.47 (2026-08-08)

Method expressions `T.M` and `(*T).M` as unbound function values on the
`uli_fn` path. Method-set rules match Go (value receivers on `T`; value
and pointer on `*T`). See `constructs/method-expr.yaml`.

## Tamil-0.48 (2026-08-08)

Concurrency on the **C** backend (pthread), not deferred to NASM:
`இழை` (go), `தடம்` (chan), `தடத்தேர்வு`/`எனில்`/`மற்றபடி` (select),
`மூடு` (close), ASCII `<-`. Select cases reuse `எனில்` (like `திசைவி`
and Go `case`); provisional `சூழல்` was dropped as too close to `சுழல்`.
Buffered and unbuffered channels; directional types. See
`constructs/goroutine.yaml`.

Parser hardening (same day): after a syntax error inside `{`…`}` loops,
always make progress (`advancePastError`) or stay on a sync token
(`}` / EOF / `;`). Select/switch/struct invalid corpus + 2s hang tests
guard against unbounded parse loops.

## Tamil-0.49 (2026-08-13)

Panic / recover: **அலறு** / **மீள்**, C `setjmp`/`longjmp` + `தள்ளிவை`
unwind (`__thread` state per `இழை`). Reserved builtins like `மூடு`.
MVP: `அலறு` takes `சரம்`; `மீள்()` returns `சரம்` (`""` if none).
Rejected **பீதி** (≈ பதிப்பி), **வெருளி** (≈ வெளி), **எச்சரி** (warn ≠ abort).
See `constructs/panic.yaml`.

## Tamil-0.50 (2026-08-13)

Panic / concurrency polish (no new keywords):
- `அலறு` accepts primitives (`முழுஎண்`, `மிதவைஎண்`, `நிலை`, `இருமி8`,
  `இருமி32`) and stringifies them; `மீள்` still returns `சரம்`.
- Uncaught `அலறு` dumps a function-name stack, then aborts.
- Panic / recover is per-`இழை` (recover in one thread does not see
  another).
- Unbuffered `தடத்தேர்வு` send only succeeds if a receiver is waiting
  (Go select); default is taken otherwise.
See `constructs/panic.yaml`, `goroutine.yaml`.

## Tamil-0.51 (2026-08-13)

`ஒவ்வொரு` over `தடம்`: at most one variable (the element); loop receives
until `மூடு` (and drain). Send-only channels are an error. Bare
`சுழல் ஒவ்வொரு x` is allowed (also for slices/maps). See `range.yaml`.

## Tamil-0.52 (2026-08-13)

Native heap + GC on the C backend (no new keywords):
- Replace bump arena with malloc-backed stop-the-world conservative
  mark-sweep (`uli_alloc`).
- Safe points: poll at function entry / loop back-edges; park around
  `தடம்` / `தடத்தேர்வு` cond-wait.
- Thread registry + Linux `pthread_getattr_np` stack bounds.
- `இழை` register/unregister; `uli_arena_alloc` is a wrapper.
See `constructs/gc.yaml`. HTTP / sockets still later.

## Tamil-0.53 (2026-08-13)

TCP sockets on the C/Linux backend via stdlib package **வலை** (no new
keywords). `கொணர் "வலை"` resolves to `stdlib/வலை`. Listen/accept/dial,
read/write `[]இருமி8`, close, local `முகவரி`. Blocking I/O parks for GC.
Failures abort. UDP / TLS / HTTP later. See `constructs/net.yaml`.

## Tamil-0.54 (2026-08-13)

Minimal HTTP/1.0 server via stdlib package **பரிமாற்றம்** (no new keywords).
Handler is a function value (`கையாளு`), not an interface. `சேவைஒன்று` for
one request; `கேட்டுசேவை` listen loop with per-conn threads. Write status +
Content-Type + body (`எழுது` / `எழுதுசரம்`). Request exposes method, path,
body. See `constructs/http.yaml`. Client / mux / TLS later.
Also: string escapes `\n` `\r` `\t` `\\` `\"` are now interpreted in the lexer
(needed for HTTP request lines).

## Tamil-0.55 (2026-08-14)

Network and HTTP operational failures return `*வலை.பிழை` rather than aborting.
`பிழை` has numeric `குறியீடு` and text `செய்தி`; `இன்மை` means success.
TCP constructors and I/O return `(value, *பிழை)`, while close, listener close,
and HTTP operations return `*பிழை`. EOF remains `(0, இன்மை)`. Runtime
invariant and allocation failures still abort.

## Tamil-0.56 (2026-08-14)

HTTP requests expose `தலைப்புகள் அகராதி[சரம்]சரம்`. Header names are
lowercased, surrounding value whitespace is trimmed, and repeated names use
the last value. `பாதை` excludes the URL query.

`பரிமாற்றம்.வழிப்படுத்தி` is an exact-path mux. `பதிவு` adds or replaces a
handler; `வழிசேவைஒன்று` and `வழிசேவை` dispatch requests and write 404 for
misses. Routes are registered before serving.

## Tamil-0.57 (2026-08-14)

Synchronous HTTP/1.0 client in `பரிமாற்றம்`: `பெறு`, `பதிவிடு`, and
`கோரு` return `(பதில், *வலை.பிழை)`. `பதில்` fields are `குறியீடு`,
`விளக்கம்` (status text — `நிலை` is the bool keyword), `தலைப்புகள்`,
and `உடல்`. Only `http://` IPv4 URLs; HTTPS/chunked/redirects/TLS out of
scope.

Custom response headers via additive `எழுதுதலைப்புகள்` /
`எழுதுசரந்தலைப்புகள்` (map-passing; `பதிலளிப்பு` stays a conn fd).
`உள்ளடக்கம்` owns Content-Type. User maps may not set Host,
Content-Length, Connection, Transfer-Encoding, or Content-Type; CR/LF
injection returns `*வலை.பிழை`.

## Tamil-0.58 (2026-08-14)

Function values may be stored in struct fields and slices, including through
function type aliases. Aggregate printing renders them as `<செயல்பாடு>` and
aggregates containing them are not comparable. The HTTP mux now uses an Niraluli
`வழிப்படுத்தி` struct with parallel path and handler slices; callers pass
`*வழிப்படுத்தி` to registration and serving functions.

## Tamil-0.59 (2026-08-14)

HTTP requests expose `வினாக்கள் அகராதி[சரம்]சரம்`. Query keys and values
decode `+` as space and valid percent escapes; repeated keys are last-wins.
Malformed escapes produce a 400 response. `பாதை` remains query-free.

The client accepts strict HTTP/1.0 or HTTP/1.1 status lines and decodes
`Transfer-Encoding: chunked` bodies up to the existing 1 MiB decoded-body
limit. Unsupported transfer encodings remain errors. The obsolete C mux
table was removed now that Tamil-0.58's mux is implemented in Niraluli.


## Tamil-0.60 (2026-08-14)

`uli build` and `uli run` compile generated C with `-O2` by default.
`--debug` / `-O0` restore unoptimized builds; explicit `-O1`, `-O2`, `-O3`,
and `-Os` are accepted.

String `+` chains evaluate operands left-to-right, measure once, and allocate
once. Empty components are allocation-free when at most one component is
non-empty. The conservative collector captures register roots with `setjmp`,
builds a sorted object index once per collection for logarithmic interior-
pointer lookup, and grows its trigger after collection when one large
allocation exceeds the normal threshold. This also fixes the prior retry loop
for allocations larger than 256 KiB on an otherwise small heap.

## Tamil-0.61 (2026-08-14)

`பரிமாற்றம்.கோருவிருப்பம்` accepts `வாடிக்கையாளர்விருப்பம்` with
`நேரமுடிவு` (milliseconds; zero means no send/receive socket timeout) and
`அதிகதிருப்புகள்` (zero disables redirects). It follows bounded absolute
`http://` 301/302/303/307/308 redirects; unsupported relative/HTTPS targets
return `*வலை.பிழை`. DNS and connection establishment are deliberately not
timed in this synchronous implementation.

`வலை` now supports IPv4 UDP with `தகவல்தளம்`, bind, send-to, receive-from,
local-address, and close operations. Datagram receives return the sender
address and truncate payloads to the caller buffer.

## Tamil-0.62 (2026-08-15)

Stdlib package **தரவுத்தளம்** introduces a shared SQL API over an internal C
backend vtable. The first backend is SQLite; `திற("sqlite", path)` and
`திற("sqlite3", path)` select it. This is intentionally a first-party backend
abstraction, not yet a public driver/plugin ABI.

Connections and prepared statements are opaque `முழுஎண்` handles. The API
supports direct no-row execution, prepared statements, typed one-based binds
(null / integer / float / UTF-8 text / bytes), row stepping, typed zero-based
column reads, reset, finalize, and close. Text and blob column values are
copied into Niraluli's managed heap before SQLite can invalidate them. Operational
failures return `*தரவுத்தளம்.பிழை`.

The generated runtime uses `dlopen` / `dlsym` for `libsqlite3.so.0` (falling
back to `libsqlite3.so`). Consequently Niraluli compilation does not require
SQLite headers or `-lsqlite3`; the shared library is required when a SQLite
program runs. Tamil-0.81 adds a PostgreSQL backend the same way (`libpq.so.5`,
drivers `"postgres"` / `"postgresql"`, `$1` placeholders). The vtable isolates
engine operations so a future MySQL first-party backend can share the Tamil API.
Tamil-0.62 deliberately excludes pooling, concurrent use of one handle,
ORM/reflection, migrations, and a third-party driver registry.

## Tamil-0.63 (2026-08-22)

Stdlib package **கோப்பு** adds POSIX file I/O on Linux. Opaque `கை`
handles are file descriptors. `திற` opens for reading; `உருவாக்கு` creates
or truncates for writing (mode 0644). `படி` / `எழுது` transfer `[]இருமி8`
buffers (EOF is `(0, இன்மை)` like network reads). `விடு` closes the handle.
`படிஅனை` / `எழுதுஅனை` read or write an entire regular file in one call;
read results are copied into the managed heap. Operational failures return
`*கோப்பு.பிழை`. Blocking I/O parks for GC. Tamil-0.78 adds seek, directories,
chmod, and directory listing (see below).

## Tamil-0.64 (2026-08-23)

Stdlib package **நேரம்** adds wall-clock time and durations on Linux.
`காலம்` is nanoseconds; `தருணம்` is a UTC Unix-nanosecond instant.
`இப்போ` uses `CLOCK_REALTIME`; `உறங்கு` sleeps via `nanosleep` (parks for
GC); `கழித்தது` is elapsed since an instant. Unit helpers
(`நானோவினாடி` / `நுண்ணியவினாடி` / `மில்லிவினாடி` / `வினாடி`) build
durations. `சரம்ஆக்கு` formats RFC3339 UTC with nanoseconds. Tamil-0.79 adds
parse, UTC/Local formatting, timers (`பின்னர்`), tickers (`துடிப்பு`), and
`நிறுத்து`.

## Tamil-0.65 (2026-08-23)

Stdlib package **வடிவம்** adds fmt-style string formatting without
variadics. `வடிவமை(வார்ப்பு, []சரம்)` substitutes `%s` in order (`%%` for
a literal percent); missing args become `%!s(MISSING)`. Scalar helpers
`எண்உரை` / `மிதவைஉரை` / `நிலைஉரை` convert to `சரம்`. `இணை` is
`strings.Join`. `வடிவமை1`/`2`/`3` wrap small fixed arities. Width,
precision, `%d`/`%g`, and writer Printf wait on richer typing or later slices.

## Tamil-0.66 (2026-08-23)

**Variadic parameters** use Go punctuation `...T` on the final parameter of
functions, methods, function literals, and function types. Inside the body the
name has type `[]T`. Call sites may pass zero or more trailing `T` values; the
emitter packs them into a temporary slice. Tamil-0.69 adds `slice...` expansion
at call sites. `வடிவம்.வடிவமை` now takes `...சரம்` (removed fixed-arity `வடிவமை1`/`2`/`3`).

## Tamil-0.67 (2026-08-23)

**Compile-time constants** use keyword `மாறிலி` (not `நிலை`, which remains
the bool type). Package- and block-level consts of scalar types with constant
expressions; `வெளி` exports package consts. Values are folded and inlined by the C emitter.
See `constructs/const.yaml` and `corpus/tamil/மாறிலி.uli`.

## Tamil-0.68 (2026-08-29)

**Const groups and iota:** parenthesized `மாறிலி ( … )` with multiple specs;
predeclared `iota` (ASCII, Go-style) increments per spec; omitted RHS and type
repeat the previous spec. See `corpus/tamil/மாறிலி_iota.uli`.

## Tamil-0.69 (2026-08-29)

**Slice expansion at variadic calls:** the final call argument may be `expr...`
where `expr` has type `[]T` and the callee takes `...T`. The slice is passed
directly as the variadic parameter value. See updated `corpus/tamil/பலவாதங்கள்.uli`.

## Tamil-0.70 (2026-08-30)

**Package-level variables:** top-level `மாறி` with optional `= init` and
optional `வெளி` export. Zero-init when no initializer; runtime init hook for
explicit values. See `corpus/tamil/மாறி_தொகுப்பு.uli`.

## Tamil-0.71 (2026-08-30)

**Generic types and constraints:** `வகை Name[யா]` struct/defined types with
`Name[T]` instantiation; union constraints `T | U`; predeclared `எதுவும்` (any).
Function constraints use the same syntax. See `corpus/tamil/பொதுவகை_வகை.uli`.

## Tamil-0.72 (2026-08-30)

**Generic methods:** methods on generic types with receivers that use the
type's type parameters (`(b பெட்டி[யா])`), not method-level `[T]`. Each
instantiation monomorphizes method bodies. See `corpus/tamil/பொதுவகை_முறை.uli`.

## Tamil-0.73–0.77 (2026-09-05)

**Language polish batch:**
- **0.73** `மாறி ( … )` groups (package + block), optional type inference
- **0.74** bitwise/shifts (`& | ^ << >> &^`, unary `^`); `1 << iota`
- **0.75** `முழுஎண்8…64` / `நேர்முழு8…64`
- **0.76** generic aliases `வகை T[யா] = …`
- **0.77** typed `வடிவமை` verbs `%d/%t/%g/%v` on constant formats

## Tamil-0.78–0.81 (2026-09-06)

**Stdlib polish batch:**
- **0.78** `கோப்பு`: `தேடு`, `அடைவுஉருவாக்கு`, `அடைவுபடி`, `அனுமதிமாற்று`
- **0.79** `நேரம்`: `பகு`, `சரம்ஆக்குஇடம்`, `பின்னர்`, `துடிப்பு`, `நிறுத்து`
- **0.80** HTTP: dial/connect timeout via `நேரமுடிவு`; relative redirect `Location`
- **0.81** `தரவுத்தளம்`: PostgreSQL backend (`postgres`/`postgresql`, libpq dlopen)

## Priority (2026-08-01)

Harden the language (other gaps) **before** starting the NASM x86-64
backend. See `subset-roadmap.md`.

## Tamil-0 freeze

| Item | Value |
|------|--------|
| Status | **Frozen** |
| Date | 2026-08-01 |
| Artifacts | `grammar/tamil/keywords.yaml`, `grammar/tamil/ebnf.md`, `grammar/tamil/constructs/*`, `corpus/tamil/*` |
| Change policy | No casual keyword/EBNF edits; bump to Tamil-0.1+ for breaking changes |

Tamil-0 is the implementation target for Phase 1 (lexer → parser → C backend).

Phase 1 progress: lexer, parser, typecheck, C emit (`emit`/`build`/`run`). NASM still later.

## Keyword locks (Tamil-0)

| Role | Tamil | Locked |
|------|--------|--------|
| package | தொகுப்பு | 2026-08-01 freeze |
| var | மாறி | 2026-07-25 |
| int type | முழுஎண் | 2026-08-01 |
| bool type | நிலை | 2026-08-01 freeze |
| true / false | மெய் / பொய் | 2026-08-01 freeze |
| short declare | `:=` (Go-style, with மாறி) | 2026-07-25 |
| const | மாறிலி | Tamil-0.67 |
| if / else | எனில் / இல்லையேல் | 2026-07-25 |
| return | திருப்பு | 2026-07-25 |
| print | பதிப்பி | 2026-07-25 |
| func | செயல்பாடு | 2026-07-25 |
| entry name | தொடக்கம் (identifier, not keyword) | 2026-07-25 |
| for | சுழல் | Tamil-0.1 |
| break | முறி | Tamil-0.1 |
| continue | தொடர் | Tamil-0.1 |
| string type | சரம் | Tamil-0.2 |
| slice type | `[]T` (ASCII brackets) | Tamil-0.3 |
| len | நீளம் | Tamil-0.3 |
| range | ஒவ்வொரு | Tamil-0.4 (slices); 0.5 also சரம் |
| append | சேர் | Tamil-0.5 |
| type | வகை | Tamil-0.6 |
| struct | அமைப்பு | Tamil-0.6 |
| pointer ops | `*` / `&` (ASCII) | Tamil-0.7 |
| methods | receiver after `செயல்பாடு` | Tamil-0.8 |
| multi-file package | same `தொகுப்பு` name, no import | Tamil-0.9 |
| import | கொணர் | Tamil-0.10 |
| default (later) | மற்றபடி | reserved |
| map | அகராதி | Tamil-0.35 |
| delete | நீக்கு | Tamil-0.35 |
| defer | தள்ளிவை | Tamil-0.37 |
| panic | அலறு | Tamil-0.49 |
| recover | மீள் | Tamil-0.49 |
