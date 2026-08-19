// Command coreupgrade rewrites imports of the packages this module deprecated
// in v1.6.0 to the paths that replace them in v2.0.0.
//
// Usage:
//
//	go run github.com/go-admin-team/go-admin-core/tools/coreupgrade@latest [-w] DIR
//
// It reports what it would change and exits. Pass -w to write the files.
package main

import (
	"flag"
	"fmt"
	"io/fs"
	"os"
	"path/filepath"
	"strings"
)

func main() {
	write := flag.Bool("w", false, "write the files instead of reporting what would change")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: coreupgrade [-w] DIR\n\n")
		fmt.Fprintf(os.Stderr, "Rewrites deprecated go-admin-core imports. Without -w it only reports.\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	n, files, err := run(flag.Arg(0), *write, os.Stdout)
	if err != nil {
		fmt.Fprintln(os.Stderr, "coreupgrade:", err)
		os.Exit(1)
	}

	switch {
	case n == 0:
		fmt.Println("no deprecated imports found")
	case *write:
		fmt.Printf("\nrewrote %d import(s) in %d file(s)\n", n, files)
	default:
		fmt.Printf("\n%d import(s) in %d file(s) would be rewritten; pass -w to apply\n", n, files)
	}
}

func run(dir string, write bool, out *os.File) (imports, files int, err error) {
	err = filepath.WalkDir(dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) && path != dir {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(path, ".go") {
			return nil
		}

		src, err := os.ReadFile(path)
		if err != nil {
			return err
		}

		next, changes, err := rewrite(path, src)
		if err != nil {
			// A file that does not parse is reported and stepped over: the
			// tool has no business deciding a consumer's build is broken.
			fmt.Fprintln(os.Stderr, "coreupgrade: skipping", err)
			return nil
		}
		if len(changes) == 0 {
			return nil
		}

		rel, relErr := filepath.Rel(dir, path)
		if relErr != nil {
			rel = path
		}
		for _, c := range changes {
			note := ""
			if c.Aliased {
				note = fmt.Sprintf(" (kept the local name %q; the package is now %q)",
					packageName(c.From), packageName(c.To))
			}
			fmt.Fprintf(out, "%s:%d: %s -> %s%s\n", rel, c.Line, c.From, c.To, note)
		}

		if write {
			if err := os.WriteFile(path, next, 0o644); err != nil {
				return err
			}
		}

		imports += len(changes)
		files++
		return nil
	})
	return imports, files, err
}

func skipDir(name string) bool {
	switch name {
	case ".git", "vendor", "node_modules", "testdata":
		return true
	}
	return false
}

func packageName(path string) string {
	for _, m := range moves {
		switch path {
		case m.from:
			return m.name
		case m.to:
			return m.newName
		}
	}
	return filepath.Base(path)
}
