// Command uli is the Niraluli compiler driver (Tamil-0 Phase 1).
//
// Capabilities: lex | parse | check | build | run | emit
package main

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"niraluli/internal/ast"
	"niraluli/internal/check"
	"niraluli/internal/emitc"
	"niraluli/internal/lex"
	"niraluli/internal/load"
	"niraluli/internal/token"
)

func main() {
	if len(os.Args) < 2 {
		usage()
		os.Exit(2)
	}

	mode := "run"
	outPath := ""
	optLevel := defaultCOpt
	var paths []string
	for i := 1; i < len(os.Args); i++ {
		a := os.Args[i]
		switch {
		case a == "-h" || a == "--help" || a == "help":
			usage()
			return
		case a == "lex" || a == "-lex":
			mode = "lex"
		case a == "parse" || a == "-parse":
			mode = "parse"
		case a == "check" || a == "-check":
			mode = "check"
		case a == "build" || a == "-build":
			mode = "build"
		case a == "emit" || a == "-emit":
			mode = "emit"
		case a == "run" || a == "-run":
			mode = "run"
		case a == "-O0" || a == "-O1" || a == "-O2" || a == "-O3" || a == "-Os":
			optLevel = a
		case a == "--debug":
			optLevel = "-O0"
		case a == "-o":
			if i+1 >= len(os.Args) {
				fmt.Fprintln(os.Stderr, "-o requires an argument")
				os.Exit(2)
			}
			i++
			outPath = os.Args[i]
		case strings.HasPrefix(a, "-"):
			fmt.Fprintf(os.Stderr, "unknown flag %s\n", a)
			usage()
			os.Exit(2)
		default:
			paths = append(paths, a)
		}
	}
	if len(paths) == 0 {
		fmt.Fprintln(os.Stderr, "missing source file or directory")
		usage()
		os.Exit(2)
	}

	expanded, err := load.Expand(paths)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
	outBase := load.DefaultOutName(expanded)
	if len(paths) == 1 {
		if st, err := os.Stat(paths[0]); err == nil && st.IsDir() {
			outBase = filepath.Base(paths[0])
		}
	}

	switch mode {
	case "lex":
		os.Exit(runLexAll(expanded))
	case "parse":
		os.Exit(runParseAll(expanded))
	case "check":
		os.Exit(runCheckAll(paths))
	case "build":
		os.Exit(runBuildAll(paths, outBase, outPath, false, optLevel))
	case "emit":
		os.Exit(runEmitAll(paths, outBase, outPath))
	case "run":
		os.Exit(runBuildAll(paths, outBase, outPath, true, optLevel))
	default:
		fmt.Fprintf(os.Stderr, "unknown mode %s\n", mode)
		os.Exit(2)
	}
}

func runLexAll(paths []string) int {
	code := 0
	for _, path := range paths {
		src, err := os.ReadFile(path)
		if err != nil {
			fmt.Fprintln(os.Stderr, err)
			code = 1
			continue
		}
		if len(paths) > 1 {
			fmt.Printf("# %s\n", path)
		}
		if c := runLex(path, string(src)); c != 0 {
			code = c
		}
	}
	return code
}

func runLex(path, src string) int {
	l := lex.New(path, src)
	for {
		tok := l.Next()
		if tok.Kind == token.EOF {
			fmt.Printf("%d:%d\t%s\n", tok.Pos.Line, tok.Pos.Col, tok.Kind)
			break
		}
		if tok.Lit != "" {
			fmt.Printf("%d:%d\t%s\t%q\n", tok.Pos.Line, tok.Pos.Col, tok.Kind, tok.Lit)
		} else {
			fmt.Printf("%d:%d\t%s\n", tok.Pos.Line, tok.Pos.Col, tok.Kind)
		}
	}
	code := 0
	for _, e := range l.Errors() {
		fmt.Fprintln(os.Stderr, e)
		code = 1
	}
	return code
}

func runParseAll(paths []string) int {
	files, errs := load.ParsePaths(paths)
	for _, e := range errs {
		fmt.Fprintln(os.Stderr, e)
	}
	if len(errs) != 0 {
		return 1
	}
	for i, f := range files {
		if len(files) > 1 {
			fmt.Printf("# %s\n", paths[i])
		}
		printFile(f)
	}
	return 0
}

func compileFront(paths []string) (*check.ProgramInfo, int) {
	prog, lerrs := load.LoadProgram(paths)
	for _, e := range lerrs {
		fmt.Fprintln(os.Stderr, e)
	}
	if len(lerrs) != 0 {
		return nil, 1
	}
	merged, merrs := prog.MergedFiles()
	for _, e := range merrs {
		fmt.Fprintln(os.Stderr, e)
	}
	if len(merrs) != 0 {
		return nil, 1
	}
	pi, cerrs := check.CheckProgram(merged, prog.Entry.Name)
	for _, e := range cerrs {
		fmt.Fprintln(os.Stderr, e)
	}
	if len(cerrs) != 0 {
		return nil, 1
	}
	return pi, 0
}

func runCheckAll(paths []string) int {
	_, code := compileFront(paths)
	if code != 0 {
		return code
	}
	fmt.Println("ok")
	return 0
}

func runEmitAll(paths []string, outBase, outPath string) int {
	pi, code := compileFront(paths)
	if code != 0 {
		return code
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cFile, err := resolveCOut(outBase, outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	fmt.Println(cFile)
	return 0
}

const defaultCOpt = "-O2"

func runBuildAll(paths []string, outBase, outPath string, run bool, optLevel string) int {
	pi, code := compileFront(paths)
	if code != 0 {
		return code
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	binPath, err := resolveBinOut(outBase, outPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	cFile := binPath + ".c"
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}

	gcc, err := exec.LookPath("gcc")
	if err != nil {
		gcc, err = exec.LookPath("cc")
	}
	if err != nil {
		fmt.Fprintln(os.Stderr, "gcc/cc not found; wrote", cFile, "only")
		fmt.Fprintln(os.Stderr, "install a C compiler (e.g. sudo apt install gcc) to build binaries")
		if run {
			return 1
		}
		return 0
	}
	cmd := exec.Command(gcc, "-std=c11", optLevel, "-pthread", cFile, "-o", binPath, "-ldl")
	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr
	if err := cmd.Run(); err != nil {
		return 1
	}
	fmt.Fprintln(os.Stderr, "wrote", cFile, "and", binPath)

	if !run {
		return 0
	}
	abs, err := filepath.Abs(binPath)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	runCmd := exec.Command(abs)
	runCmd.Stdout = os.Stdout
	runCmd.Stderr = os.Stderr
	if err := runCmd.Run(); err != nil {
		if ee, ok := err.(*exec.ExitError); ok {
			return ee.ExitCode()
		}
		fmt.Fprintln(os.Stderr, err)
		return 1
	}
	return 0
}

const defaultBuildDir = "build"

func ensureParentDir(path string) error {
	dir := filepath.Dir(path)
	if dir == "." || dir == "" {
		return nil
	}
	return os.MkdirAll(dir, 0o755)
}

func resolveBinOut(outBase, outPath string) (string, error) {
	if outPath == "" {
		outPath = filepath.Join(defaultBuildDir, outBase)
	}
	if err := ensureParentDir(outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

func resolveCOut(outBase, outPath string) (string, error) {
	if outPath == "" {
		outPath = filepath.Join(defaultBuildDir, outBase+".c")
	} else if !strings.HasSuffix(outPath, ".c") {
		outPath = outPath + ".c"
	}
	if err := ensureParentDir(outPath); err != nil {
		return "", err
	}
	return outPath, nil
}

func printFile(f *ast.File) {
	fmt.Printf("package %s\n", f.Package.Name.Name)
	for _, im := range f.Imports {
		path := ""
		if im.Path != nil {
			path = im.Path.Value
		}
		fmt.Printf("import %q\n", path)
	}
	for _, d := range f.Decls {
		switch d := d.(type) {
		case *ast.FuncDecl:
			res := ""
			switch len(d.Results) {
			case 0:
			case 1:
				if d.Results[0].Name != nil {
					res = " (" + d.Results[0].Name.Name + " " + ast.TypeString(d.Results[0].Type) + ")"
				} else {
					res = " " + ast.TypeString(d.Results[0].Type)
				}
			default:
				res = " ("
				for i, r := range d.Results {
					if i > 0 {
						res += ", "
					}
					if r.Name != nil {
						res += r.Name.Name + " "
					}
					res += ast.TypeString(r.Type)
				}
				res += ")"
			}
			recv := ""
			if d.Recv != nil {
				recv = "(" + d.Recv.Name.Name + " " + ast.TypeString(d.Recv.Type) + ") "
			}
			fmt.Printf("func %s%s(%d params)%s { %d stmts }\n", recv, d.Name.Name, len(d.Params), res, len(d.Body.List))
		case *ast.VarDecl:
			fmt.Printf("var %d names type %s\n", len(d.Names), ast.TypeString(d.Type))
		case *ast.VarGroupDecl:
			fmt.Printf("var group %d specs\n", len(d.Specs))
		case *ast.TypeDecl:
			fmt.Printf("type %s\n", d.Name.Name)
		default:
			fmt.Printf("decl %T\n", d)
		}
	}
}

func usage() {
	fmt.Fprintf(os.Stderr, `uli — Niraluli language toolchain (Tamil-0)

Usage:
  uli [run|build|emit|check|parse|lex] [-O0|-O1|-O2|-O3|-Os|--debug] [-o out] <file.uli|dir>…

Commands:
  run     typecheck, emit C, compile with gcc/cc, and execute (default)
  build   typecheck, emit C, compile (do not run)
  emit    typecheck and write C only
  check   parse + typecheck only
  parse   parse and print a short AST summary
  lex     tokenize and print tokens

C compilation defaults to -O2; use --debug or -O0 for unoptimized output.

Packages: pass a directory or .uli files. Use கொணர் "rel/dir" to import
another package directory; call as pkg.Name (Tamil-0.10).

Examples:
  uli corpus/tamil/வணக்கம்.uli
  uli corpus/tamil/பல்கோப்பு
  uli corpus/tamil/கொணர்_எடு
`)
}
