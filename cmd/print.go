package cmd

import (
	"fmt"
	"os"
)

func PrintTaskOutputf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stdout, format, a...)
}

func PrintTaskErrorf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
}

func PrintAppOutputf(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	//flush
	os.Stderr.Sync()
}

func PrintAppErrorfAndExit(format string, a ...interface{}) {
	fmt.Fprintf(os.Stderr, format, a...)
	os.Exit(1)
}
