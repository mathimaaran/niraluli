# Niraluli (நிரலுளி)

Niraluli is a **Go-inspired** programming language with **semantic Tamil** keywords.
It is free to diverge from Go. The long-term goal is a quality compiler for
Linux (x86-64 first, then i386), with NASM as a later backend.

**நிரலுளி** joins **நிரல்** (program) and **உளி** (chisel): a tool for carving programs.

**Phase 0:** complete (Tamil-0 frozen 2026-08-01).  
**Phase 1:** active C backend through v0.77; NASM remains later.

| Item | Value |
|------|--------|
| Language | Niraluli (நிரலுளி) |
| Host compiler language | Go |
| Source extension | `.uli` |
| Keywords | Semantic Tamil |
| Semantics | Go-inspired, free to diverge |
| Language subset | **v0.77** |
| Early backend | Emit C (reuse `gcc`/`clang`) |
| Later backend | NASM → Linux ELF (64-bit, then 32-bit) |
| Status | Experimental research compiler (Linux + C backend) |
| License | [Apache License 2.0](LICENSE) |

## First program

```text
corpus/tamil/வணக்கம்.uli
```

## After downloading (release zip or clone)

Niraluli ships as **source**, not a prebuilt installer. Unzip or clone on **Linux**, then install only what you need for the path below.

### Always required (compile and run Niraluli)

| Need | Why | Typical install (Ubuntu/Debian) |
|------|-----|----------------------------------|
| Linux | Supported runtime target | — |
| Go toolchain | Hosts the Niraluli compiler (`cmd/uli`) | System Go, or `source tools/env.sh` / `./tools/run-go.sh` |
| GCC or Clang (`cc`) | Compiles generated C | `sudo apt install build-essential` |
| pthread + `libdl` (`-ldl`) | Runtime / SQL dynamic loading | Usually included with the toolchain |

Minimal try-out:

```bash
cd uli-0.62   # or your unzipped / cloned directory name
source tools/env.sh   # only if you do not already have `go` on PATH
go run ./cmd/uli run corpus/tamil/வணக்கம்.uli
```

Open [`docs/index.html`](docs/index.html) in a browser for the developer docs.

### Optional: formatting (`வடிவம்`)

```bash
go run ./cmd/uli run corpus/tamil/வடிவமைப்பு/
```

### Optional: time (`நேரம்`)

```bash
go run ./cmd/uli run corpus/tamil/நேரம்/
```

### Optional: file I/O (`கோப்பு`)

```bash
go run ./cmd/uli run corpus/tamil/கோப்பு/
```

### Optional: SQL programs (`தரவுத்தளம்`)

| Need | Why |
|------|-----|
| Runtime `libsqlite3.so.0` | Loaded at **run** time via `dlopen` |

```bash
sudo apt install libsqlite3-0
# check: ldconfig -p | grep libsqlite3
go run ./cmd/uli run corpus/tamil/தரவுத்தளம்/
```

You do **not** need SQLite headers, `libsqlite3-dev`, or `-lsqlite3` to build.

### Optional: full-stack employee demo only

Extra on top of Go + C + SQLite — **not** required for normal Niraluli use:

| Need | Why |
|------|-----|
| Node.js **18+** and npm | Vite React UI in `corpus/tamil/ஊழியர்_முழுஅடுக்கு/` |

Follow [`corpus/tamil/ஊழியர்_முழுஅடுக்கு/SETUP.md`](corpus/tamil/ஊழியர்_முழுஅடுக்கு/SETUP.md).

### Not required for core Niraluli

- A prebuilt `uli` binary (use `go run` / `go build ./cmd/uli`)
- Node / npm (except the React demo above)
- SQLite `-dev` packages
- Docker, Python, Java, etc.

## Layout

```text
notes/           design decisions, roadmap, Unicode, Tamil fonts
grammar/go/      frozen Go reference (inspiration, not a checklist)
grammar/tamil/   Niraluli grammar, keywords, construct cards
corpus/tamil/    example and test programs
stdlib/          compiler stdlib (வலை, பரிமாற்றம், தரவுத்தளம், கோப்பு, நேரம், வடிவம்)
tools/           portable Go helpers (`env.sh`, `run-go.sh`)
docs/            static HTML developer documentation
```

## Editor setup (Tamil glyphs)

Niraluli source uses Tamil script. If you see **squares** instead of letters, install/use
**Noto Sans Tamil** and set a font fallback — see [`notes/fonts-tamil.md`](notes/fonts-tamil.md).

## Status

- Phase 0 complete — Tamil-0 **frozen** (2026-08-01). See `notes/design-decisions.md`.
- Phase 1: lexer + parser + typecheck + **C emit/run** through **v0.77** on Linux.
- Suitable for **public experimental** use if framed as early/research; not a polished production language yet.
- Next major backend: NASM x86-64. Optional leftovers live in [`notes/deferred.md`](notes/deferred.md).

```bash
source tools/env.sh   # if needed
go test ./...
go run ./cmd/uli check corpus/tamil/வணக்கம்.uli
go run ./cmd/uli run corpus/tamil/வணக்கம்.uli
# → build/வணக்கம்.c and build/வணக்கம் (override with -o)
# C builds default to -O2; pass --debug or -O0 for an unoptimized build.
```

See [`tools/README.md`](tools/README.md) for the portable Go toolchain.

## Developer documentation

Open [`docs/index.html`](docs/index.html) for the comprehensive static HTML
reference: getting started, language syntax, concurrency/runtime, standard
library APIs (TCP/UDP, HTTP, SQL/`தரவுத்தளம்`, files/`கோப்பு`, time/`நேரம்`, formatting/`வடிவம்`), compiler tooling,
implementation notes, and misc extras (license and Tamil typing tutors
without phonetic input).

## License

Niraluli is licensed under the [Apache License, Version 2.0](LICENSE).
See [`NOTICE`](NOTICE) for copyright attribution.

## Working with AI agents

Prefer feeding: construct card(s) + keyword slice + a few corpus examples +
explicit subset (`Tamil-0`), not the entire Go grammar.
