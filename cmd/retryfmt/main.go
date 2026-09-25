// Command retryfmt validates and reformats retry policy files.
//
// Usage:
//
//	retryfmt [-l] [-w] [file ...]
//
// With no files, retryfmt reads a single policy from stdin and writes its
// canonical form to stdout. With one or more files, it validates each one
// and, unless -l or -w is given, writes the canonical form of each to
// stdout in turn.
//
//	-l	list files whose formatting differs from canonical, without
//		writing anything
//	-w	overwrite each file with its canonical form instead of
//		printing it
//
// -l and -w can be combined, matching gofmt: files are listed and rewritten
// in the same pass.
//
// A file that fails to parse is reported on stderr as "name: message" and
// retryfmt keeps going with the remaining files, exiting with status 1 once
// all of them have been processed.
package main

import (
	"flag"
	"fmt"
	"io"
	"os"

	retrypolicy "github.com/heathpebble667/retry-policy-spec"
)

func main() {
	list := flag.Bool("l", false, "list files whose formatting differs from canonical")
	write := flag.Bool("w", false, "overwrite each file with its canonical form")
	flag.Usage = usage
	flag.Parse()

	args := flag.Args()
	if len(args) == 0 {
		if *list || *write {
			fmt.Fprintln(os.Stderr, "retryfmt: -l and -w require at least one file")
			os.Exit(2)
		}
		if err := formatStdin(); err != nil {
			fmt.Fprintf(os.Stderr, "retryfmt: %v\n", err)
			os.Exit(1)
		}
		return
	}

	failed := false
	for _, name := range args {
		if err := processFile(name, *list, *write); err != nil {
			fmt.Fprintf(os.Stderr, "%s: %v\n", name, err)
			failed = true
		}
	}
	if failed {
		os.Exit(1)
	}
}

func usage() {
	fmt.Fprintln(os.Stderr, "usage: retryfmt [-l] [-w] [file ...]")
	flag.PrintDefaults()
}

func formatStdin() error {
	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		return fmt.Errorf("reading stdin: %w", err)
	}
	policy, err := retrypolicy.Parse(string(input))
	if err != nil {
		return err
	}
	_, err = fmt.Print(policy.String())
	return err
}

func processFile(name string, list, write bool) error {
	info, err := os.Stat(name)
	if err != nil {
		return err
	}
	input, err := os.ReadFile(name)
	if err != nil {
		return err
	}
	policy, err := retrypolicy.Parse(string(input))
	if err != nil {
		return err
	}

	canonical := policy.String()
	changed := canonical != string(input)

	if list && changed {
		fmt.Println(name)
	}
	if write && changed {
		if err := os.WriteFile(name, []byte(canonical), info.Mode().Perm()); err != nil {
			return err
		}
	}
	if !list && !write {
		fmt.Print(canonical)
	}
	return nil
}
