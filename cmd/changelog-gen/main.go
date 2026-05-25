// Command changelog-gen reads commit subjects (one per line) from stdin and
// prints a single Keep-a-Changelog block for the given version. Used in CI to
// build the develop channel changelog from `git log --format='%s' <range>`.
package main

import (
	"bufio"
	"flag"
	"fmt"
	"os"
)

func main() {
	version := flag.String("version", "", "version for the changelog heading, e.g. 2.11.2+r95")
	date := flag.String("date", "", "release date YYYY-MM-DD")
	flag.Parse()

	if *version == "" || *date == "" {
		fmt.Fprintln(os.Stderr, "usage: changelog-gen --version <ver> --date <YYYY-MM-DD> < subjects")
		os.Exit(2)
	}

	var subjects []string
	sc := bufio.NewScanner(os.Stdin)
	sc.Buffer(make([]byte, 64*1024), 1024*1024)
	for sc.Scan() {
		subjects = append(subjects, sc.Text())
	}
	if err := sc.Err(); err != nil {
		fmt.Fprintln(os.Stderr, "read stdin:", err)
		os.Exit(1)
	}

	fmt.Print(Generate(subjects, *version, *date))
}
