package load_test

import (
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"niraluli/internal/check"
	"niraluli/internal/emitc"
	"niraluli/internal/load"
)

func root(t *testing.T) string {
	t.Helper()
	_, f, _, ok := runtime.Caller(0)
	if !ok {
		t.Fatal("caller")
	}
	return filepath.Join(filepath.Dir(f), "..", "..")
}

func TestExpandDir(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "பல்கோப்பு")
	paths, err := load.Expand([]string{dir})
	if err != nil {
		t.Fatal(err)
	}
	if len(paths) != 2 {
		t.Fatalf("got %d paths: %v", len(paths), paths)
	}
}

func TestMultiFileProgram(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "பல்கோப்பு")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O0", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	want := "7\n10\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestImportProgram(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "கொணர்_எடு")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	if len(prog.All) != 2 {
		t.Fatalf("packages: %d", len(prog.All))
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O0", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	want := "7\n5\nவணக்கம்\n8\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestUnexportedImportRejected(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "invalid", "தனிய_கொணர்")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	_, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) == 0 {
		t.Fatal("expected unexported name error")
	}
}

func TestUnusedImportRejected(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "invalid", "பயன்படா_கொணர்")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	_, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) == 0 {
		t.Fatal("expected unused import error")
	}
	found := false
	for _, e := range errs {
		if strings.Contains(e.Error(), "imported and not used") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("errs=%v", errs)
	}
}

func TestImportCycleRejected(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "invalid", "கொணர்_சுழற்சி")
	_, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) == 0 {
		t.Fatal("expected import cycle error")
	}
	found := false
	for _, e := range lerrs {
		if strings.Contains(e.Error(), "import cycle") {
			found = true
			break
		}
	}
	if !found {
		t.Fatalf("errs=%v", lerrs)
	}
}

func TestImportAlias(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "கொணர்_மாற்று")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O0", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatal(err)
	}
	want := "7\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestNetEchoProgram(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "வலை_எதிரொலி")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	if len(prog.All) != 2 {
		t.Fatalf("packages: %d", len(prog.All))
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cSrc, "uli_net_listen") {
		t.Fatalf("missing uli_net_listen\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O0", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "நிரலுளி\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestUDPProgram(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "தகவல்_வலை")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cSrc, "uli_net_udp_recv") {
		t.Fatalf("missing UDP runtime\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	if string(got) != "ping\npong\n" {
		t.Fatalf("got %q want UDP echo\nC:\n%s", got, cSrc)
	}
}

func TestDatabaseProgram(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "தரவுத்தளம்")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cSrc, "uli_db_vtable") || !strings.Contains(cSrc, "uli_sqlite_vtable") {
		t.Fatalf("missing database vtable runtime\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	if strings.Contains(string(got), "SQLite runtime unavailable") {
		t.Skipf("SQLite runtime unavailable: %s", got)
	}
	want := "5\nஎண்\n1\nநிரலுளி\n9.5\nதரவு\nமெய்\nunsupported database driver\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestFileProgram(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "கோப்பு")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cSrc, "uli_file_read_all") || !strings.Contains(cSrc, "uli_file_write_all") {
		t.Fatalf("missing file I/O runtime\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	cmd := exec.Command(bin)
	cmd.Dir = tmp
	got, err := cmd.CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	out := string(got)
	wantPrefix := "வணக்கம்\nநிரலுளி\n"
	if !strings.HasPrefix(out, wantPrefix) {
		t.Fatalf("got %q want prefix %q\nC:\n%s", out, wantPrefix, cSrc)
	}
	if len(out) <= len(wantPrefix) {
		t.Fatalf("missing open-error message: %q\nC:\n%s", out, cSrc)
	}
}

func TestTimeProgram(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "நேரம்")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cSrc, "uli_time_now_unix_nano") || !strings.Contains(cSrc, "uli_time_sleep") {
		t.Fatalf("missing time runtime\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "சரி\nயூனிக்ஸ்\nவடிவம்\n42\n1000\n1000000000\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestFmtProgram(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "வடிவமைப்பு")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cSrc, "uli_fmt_sprintf") || !strings.Contains(cSrc, "uli_fmt_join") {
		t.Fatalf("missing fmt runtime\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "வணக்கம், நிரலுளி!\nஎண்=42 நிலை=மெய்\nமிதவை=1.5\nசதவீதம் %s = ok\nஅ-ஆ-இ\nmissing only %!s(MISSING)\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestVariadicProgram(t *testing.T) {
	path := filepath.Join(root(t), "corpus", "tamil", "பலவாதங்கள்.uli")
	prog, lerrs := load.LoadProgram([]string{path})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "0\n6\n30\n15\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestConstProgram(t *testing.T) {
	path := filepath.Join(root(t), "corpus", "tamil", "மாறிலி.uli")
	prog, lerrs := load.LoadProgram([]string{path})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "20\nவணக்கம்\n1\n2\nமெய்\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestConstIotaProgram(t *testing.T) {
	path := filepath.Join(root(t), "corpus", "tamil", "மாறிலி_iota.uli")
	prog, lerrs := load.LoadProgram([]string{path})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "0\n1\n4\n0\n10\n20\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestPkgVarProgram(t *testing.T) {
	path := filepath.Join(root(t), "corpus", "tamil", "மாறி_தொகுப்பு.uli")
	prog, lerrs := load.LoadProgram([]string{path})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "42\n1\nநிரலுளி\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestGenericTypeProgram(t *testing.T) {
	path := filepath.Join(root(t), "corpus", "tamil", "பொதுவகை_வகை.uli")
	prog, lerrs := load.LoadProgram([]string{path})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "42\nநிரலுளி\n7\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestGenericMethodProgram(t *testing.T) {
	path := filepath.Join(root(t), "corpus", "tamil", "பொதுவகை_முறை.uli")
	prog, lerrs := load.LoadProgram([]string{path})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		for _, e := range errs {
			t.Log(e)
		}
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin, "-ldl").CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	want := "7\n42\nநிரலுளி\n"
	if string(got) != want {
		t.Fatalf("got %q want %q\nC:\n%s", got, want, cSrc)
	}
}

func TestHTTPRedirectOptions(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "வலை_திருப்பு")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cSrc, "uli_http_do_opts") {
		t.Fatalf("missing HTTP options runtime\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O2", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	if string(got) != "200\nதிருப்பியது\n" {
		t.Fatalf("got %q want redirected response\nC:\n%s", got, cSrc)
	}
}

func TestHttpServeOne(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "வலைபரிமாற்றம்")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if strings.Contains(cSrc, "uli_http_mux_serve_one") {
		t.Fatalf("obsolete C HTTP mux runtime emitted\n%s", cSrc)
	}
	if !strings.Contains(cSrc, "uli_http_headers_set") {
		t.Fatalf("missing request-header map bridge\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O0", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	out := string(got)
	if !strings.Contains(out, "HTTP/1.0 200") {
		t.Fatalf("missing status line: %q\nC:\n%s", out, cSrc)
	}
	if !strings.Contains(out, "வணக்கம்") {
		t.Fatalf("missing body: %q\nC:\n%s", out, cSrc)
	}
	if !strings.Contains(out, "uli lang\n") || !strings.Contains(out, "a/b\n") {
		t.Fatalf("missing decoded query values: %q\nC:\n%s", out, cSrc)
	}
	if !strings.Contains(out, "HTTP/1.0 404") || !strings.Contains(out, "காணவில்லை") {
		t.Fatalf("missing mux 404: %q\nC:\n%s", out, cSrc)
	}
	if strings.Contains(out, "பழையது") {
		t.Fatalf("duplicate route did not replace handler: %q\nC:\n%s", out, cSrc)
	}
}

func TestHttpHTML(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "வலைபரிமாற்றம்_html")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O0", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	out := string(got)
	if !strings.Contains(out, "text/html") {
		t.Fatalf("missing content-type: %q", out)
	}
	if !strings.Contains(out, "<h1>நிரலுளி</h1>") {
		t.Fatalf("missing html body: %q", out)
	}
}

func TestHttpClientAndResponseHeaders(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "வலை_வாடிக்கையாளர்")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(cSrc, "uli_http_do") {
		t.Fatalf("missing HTTP client runtime\n%s", cSrc)
	}
	if !strings.Contains(cSrc, "uli_http_headers_each") {
		t.Fatalf("missing header foreach bridge\n%s", cSrc)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O0", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	out := string(got)
	for _, want := range []string{"200\n", "200 OK\n", "ok\n", "நிரலுளி\n", "hello\n", "hello world\n"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q\nC:\n%s", want, out, cSrc)
		}
	}
}

func TestHttpClientErrors(t *testing.T) {
	dir := filepath.Join(root(t), "corpus", "tamil", "வலை_வாடிக்கையாளர்_பிழை")
	prog, lerrs := load.LoadProgram([]string{dir})
	if len(lerrs) != 0 {
		t.Fatal(lerrs)
	}
	merged, merrs := prog.MergedFiles()
	if len(merrs) != 0 {
		t.Fatal(merrs)
	}
	pi, errs := check.CheckProgram(merged, prog.Entry.Name)
	if len(errs) != 0 {
		t.Fatal(errs)
	}
	cSrc, err := emitc.EmitProgram(pi)
	if err != nil {
		t.Fatal(err)
	}
	cc, err := exec.LookPath("gcc")
	if err != nil {
		cc, err = exec.LookPath("cc")
		if err != nil {
			t.Skip("gcc/cc not available")
		}
	}
	tmp := t.TempDir()
	cFile := filepath.Join(tmp, "out.c")
	bin := filepath.Join(tmp, "out")
	if err := os.WriteFile(cFile, []byte(cSrc), 0o644); err != nil {
		t.Fatal(err)
	}
	if out, err := exec.Command(cc, "-std=c11", "-O0", "-pthread", cFile, "-o", bin).CombinedOutput(); err != nil {
		t.Fatalf("cc: %v\n%s\n%s", err, out, cSrc)
	}
	got, err := exec.Command(bin).CombinedOutput()
	if err != nil {
		t.Fatalf("run: %v\n%s\nC:\n%s", err, got, cSrc)
	}
	out := string(got)
	for _, want := range []string{"HTTPS not supported", "invalid HTTP URL", "reserved HTTP header"} {
		if !strings.Contains(out, want) {
			t.Fatalf("missing %q in %q\nC:\n%s", want, out, cSrc)
		}
	}
	if strings.Contains(out, "\nleak\n") || strings.HasSuffix(out, "leak\n") {
		t.Fatalf("reserved header write still sent body: %q", out)
	}
}
