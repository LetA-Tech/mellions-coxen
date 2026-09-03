// Mellions Engineer
// Built and maintained by LetA Tech Ltd.
// Contact: leta@letatech.ca

// Command releasepack creates one deterministic Mellions release archive.
package main

import (
	"flag"
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/LetA-Tech/mellions-coxen/internal/releasearchive"
)

func main() {
	root := flag.String("root", "", "directory containing one staged release package")
	output := flag.String("output", "", "archive path to write")
	epochText := flag.String("epoch", "", "Unix timestamp used for every archive header")
	flag.Parse()
	if *root == "" || *output == "" || *epochText == "" || flag.NArg() != 0 {
		fmt.Fprintln(os.Stderr, "usage: releasepack -root DIR -output FILE -epoch UNIX_SECONDS")
		os.Exit(2)
	}
	epoch, err := strconv.ParseInt(*epochText, 10, 64)
	if err != nil || epoch < 0 {
		fmt.Fprintf(os.Stderr, "invalid epoch %q\n", *epochText)
		os.Exit(2)
	}
	if err := releasearchive.Build(*root, *output, time.Unix(epoch, 0)); err != nil {
		fmt.Fprintln(os.Stderr, err)
		os.Exit(1)
	}
}
