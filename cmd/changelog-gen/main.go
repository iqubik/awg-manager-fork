// Command changelog-gen reads commit subjects/bodies from stdin and prints a
// single Keep-a-Changelog block for the given version.
package main

import (
	"flag"
	"fmt"
	"io"
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

	input, err := io.ReadAll(os.Stdin)
	if err != nil {
		fmt.Fprintln(os.Stderr, "read stdin:", err)
		os.Exit(1)
	}

	fmt.Print(Generate(ParseCommits(string(input)), *version, *date))
}
