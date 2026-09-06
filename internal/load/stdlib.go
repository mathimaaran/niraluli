package load

import (
	"fmt"
	"os"
	"path/filepath"
	"runtime"
)

// StdlibRoot is the compiler stdlib directory (repo stdlib/).
func StdlibRoot() string {
	if root := os.Getenv("NIRALULI_ROOT"); root != "" {
		return filepath.Clean(filepath.Join(root, "stdlib"))
	}
	if exe, err := os.Executable(); err == nil {
		// Prefer install layout: <root>/bin/uli → <root>/stdlib
		cand := filepath.Clean(filepath.Join(filepath.Dir(exe), "..", "stdlib"))
		if st, err := os.Stat(cand); err == nil && st.IsDir() {
			return cand
		}
	}
	_, file, _, ok := runtime.Caller(0)
	if !ok {
		return ""
	}
	return filepath.Clean(filepath.Join(filepath.Dir(file), "..", "..", "stdlib"))
}

func resolveImportDir(base, rel string) (string, error) {
	local := filepath.Clean(filepath.Join(base, rel))
	if st, err := os.Stat(local); err == nil && st.IsDir() {
		return local, nil
	}
	if rel != "" && !filepath.IsAbs(rel) && filepath.ToSlash(rel) == filepath.Clean(rel) {
		std := filepath.Clean(filepath.Join(StdlibRoot(), rel))
		if st, err := os.Stat(std); err == nil && st.IsDir() {
			return std, nil
		}
		if cwd, err := os.Getwd(); err == nil {
			std = filepath.Clean(filepath.Join(cwd, "stdlib", rel))
			if st, err := os.Stat(std); err == nil && st.IsDir() {
				return std, nil
			}
		}
	}
	return "", fmt.Errorf("not found")
}
