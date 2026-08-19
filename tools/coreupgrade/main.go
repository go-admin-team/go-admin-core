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
	"io"
	"io/fs"
	"os"
	"path"
	"path/filepath"
	"strings"
)

func main() {
	write := flag.Bool("w", false, "write the files instead of reporting what would change")
	major := flag.Bool("v2", false, "also move every import of this module onto its v2 path")
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "usage: coreupgrade [-w] [-v2] DIR\n\n")
		fmt.Fprintf(os.Stderr, "Rewrites deprecated go-admin-core imports. Without -w it only reports.\n")
		fmt.Fprintf(os.Stderr, "With -v2 it also moves every import of this module onto the v2 path,\n")
		fmt.Fprintf(os.Stderr, "which Go requires from that major version on. Run go mod tidy after.\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if flag.NArg() != 1 {
		flag.Usage()
		os.Exit(2)
	}

	n, files, err := run(flag.Arg(0), *write, *major, os.Stdout)
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

func run(dir string, write, toMajor bool, out io.Writer) (imports, files int, err error) {
	err = filepath.WalkDir(dir, func(filePath string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			if skipDir(d.Name()) && filePath != dir {
				return fs.SkipDir
			}
			return nil
		}
		if !strings.HasSuffix(filePath, ".go") {
			return nil
		}

		src, err := os.ReadFile(filePath)
		if err != nil {
			return err
		}

		next, changes, err := rewrite(filePath, src, toMajor)
		if err != nil {
			// A file that does not parse is reported and stepped over: the
			// tool has no business deciding a consumer's build is broken.
			fmt.Fprintln(os.Stderr, "coreupgrade: skipping", err)
			return nil
		}
		if len(changes) == 0 {
			return nil
		}

		rel, relErr := filepath.Rel(dir, filePath)
		if relErr != nil {
			rel = filePath
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
			// The consumer's permission bits are theirs. A migration that
			// quietly turns 0600 into 0644 is the kind of change nobody
			// thinks to look for.
			mode := fs.FileMode(0o644)
			if info, err := d.Info(); err == nil {
				mode = info.Mode().Perm()
			}
			if err := os.WriteFile(filePath, next, mode); err != nil {
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

// packageName is asked about import paths, which are slash separated on every
// platform, so it uses path rather than filepath.
func packageName(importPath string) string {
	for _, m := range moves {
		switch importPath {
		case m.from:
			return m.name
		case m.to:
			return m.newName
		}
	}
	return path.Base(importPath)
}
