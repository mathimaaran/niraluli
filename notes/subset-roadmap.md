# Subset roadmap

Grow the language only when the previous subset has a closed loop:
**grammar card → corpus → (later) parse → typecheck → backend → run**.

## Tamil-0 (first closed loop) — **FROZEN 2026-08-01**

Goal: one runnable greeting and simple integer programs via **C backend**.

Spec artifacts are frozen: `grammar/tamil/keywords.yaml`, `ebnf.md`, construct cards, corpus.
Implementation (Phase 1) may begin against this snapshot.

In scope:

- `தொகுப்பு` (package) + entry name `தொடக்கம்` (start/main, conventional identifier)
- `செயல்பாடு` (func) with no params / `முழுஎண்` (int) params later in Tamil-0.1
- integer literals, `+ - * /`, comparisons
- `மாறி` (var) and Go-style `:=` short declaration
- `எனில்` / `இல்லையேல்` (if / else)
- `திருப்பு` (return)
- `பதிப்பி` (print) as a builtin or tiny runtime helper
- string literals (for `பதிப்பி` only)

Out of scope for Tamil-0:

- floats, arrays, structs, pointers
- packages/imports beyond a single file
- methods, interfaces, generics
- concurrency, defer, panic
- NASM backend

## After Tamil-0 — Tamil-0.1 … 0.6 done

- [x] `சுழல்` / loops (Go-style single `for`)
- [x] `முறி` / break
- [x] `தொடர்` / continue
- [x] Strings as values (`சரம்`) — concat `+`, compare `==`/`!=`
- [x] Go-like slices (`[]T`, index, `நீளம்`)
- [x] `ஒவ்வொரு` (range) over slices
- [x] `சேர்` (append), `xs[i:j]`, range over `சரம்`, string arena
- [x] Named structs (`வகை` / `அமைப்பு`) — no pointers yet
- [x] Pointers (`*T`, `&x`, `*p`; auto-deref on `*struct`)
- [x] Methods on struct / pointer receivers
- [x] Same-package multi-file (`uli <dir>` / multiple `.uli`)
- [x] Cross-package `கொணர்` (import)
- [x] Export / privacy (`வெளி`)
- [x] `இன்மை` (nil) + pointer `==` / `!=`
- [x] Richer struct fields (`*T`, nested, `[]T`)
- [x] `திசைவி` / `மற்றபடி` (switch / default)
- [x] Init before `எனில்` / `திசைவி`
- [x] Struct `==` / `!=`
- [x] `பதிப்பி` whole structs
- [x] Import aliases (`கொணர்`)
- [x] Multi-`சேர்` / `xs[i:j:k]`
- [x] Positional struct literals
- [x] Nested slices (`[][]T`)
- [x] `ஆக்கு` / `நகல்` / `திறன்` (make / copy / cap)
- [x] Multiple return values
- [x] Fixed arrays (`[N]T`)
- [x] `[]struct` / `[]*T`
- [x] Named results
- [x] Naked `திருப்பு`
- [x] Parallel assign / swap
- [x] Type alias + defined `வகை`
- [x] Floats (`மிதவைஎண்`)
- [x] Tamil digits in literals
- [x] Unused imports + cycle path messages
- [x] Conversions `T(x)`
- [x] Conversion extras + mixed int/float
- [x] Maps (`அகராதி` / `நீக்கு`)
- [x] Map literals
- [x] `தள்ளிவை` (defer)
- [x] Richer map keys (comparable)
- [x] `இருமி8` / `இருமி32` + `சரம்` ↔ `[]` conversions
- [x] Function generics MVP (`[யா]`, monomorphize)
- [x] Generics polish (multi-param, `[]யா`, import)
- [x] Bare zero-arg `தள்ளிவை` sugar
- [x] Defer method bind (pointer `&x` capture)
- [x] Function types + method / package function values
- [x] Function values in struct and slice fields
- [x] Function literals / closures (capture by reference)
- [x] Go 1.22+ per-iteration `சுழல்` / `ஒவ்வொரு` vars
- [x] Method expressions `T.M` / `(*T).M`
- [x] Concurrency on C (`இழை` / `தடம்` / `தடத்தேர்வு`)
- [x] Panic / recover (`அலறு` / `மீள்`) on C (`setjmp` / `longjmp`)
- [x] Panic / concurrency polish (typed `அலறு`, traces, unbuffered select)
- [x] `ஒவ்வொரு` over `தடம்`
- [x] Native heap + GC on C (STW conservative mark-sweep)
- [x] TCP sockets (`கொணர் "வலை"`) on C/Linux
- [x] HTTP server (`கொணர் "பரிமாற்றம்"`) on C/Linux
- [x] Net / HTTP error returns (`*வலை.பிழை`)
- [x] HTTP request headers + exact-path mux
- [x] HTTP client + custom response headers
- [x] Query parameters + HTTP/1.x protocol polish
- [x] GC/string runtime polish + `-O2` default
- [x] HTTP client timeout/redirect options + IPv4 UDP
- [x] Shared SQL database vtable + SQLite backend
- [x] POSIX file I/O (`கொணர் "கோப்பு"`)
- [x] Time / duration (`கொணர் "நேரம்"`)
- [x] fmt-style formatting (`கொணர் "வடிவம்"`)
- [x] Variadic parameters (`...T`)
- [x] Compile-time constants (`மாறிலி`)
- [x] Const groups and `iota`
- [x] Slice `...` expansion at variadic call sites
- [x] Package-level variables (`மாறி`)
- [x] Generic types and union constraints
- [x] Generic methods on generic types
- [x] Package-level / block `மாறி ( … )` groups
- [x] Bitwise and shift operators
- [x] Richer integer widths
- [x] Generic type aliases
- [x] Typed fmt verbs on constant formats

## Later growth

**Native backends (next):**

5. NASM x86-64
6. i386 Linux

**Backlog leftovers** (not required before NASM; see [`deferred.md`](deferred.md)):

- Dial / DNS timeouts
- Relative HTTP redirects
- HTTPS / TLS
- Additional SQL backends (PostgreSQL / MySQL) behind the vtable
- Database pooling and a public driver/plugin ABI
- Interfaces (only if earned)
- Larger Go-inspired features only if they earn their place
  (richer generics/constraints, …)

Inventory: [`deferred.md`](deferred.md).

## Status legend (construct cards)

`planned` → `specified` → `lexed` → `parsed` → `typed` → `codegen`
