package install

import (
	"fmt"
	"os"
)

var (
	colorGreen  = "\033[32m"
	colorRed    = "\033[31m"
	colorYellow = "\033[33m"
	colorReset  = "\033[0m"
)

func init() {
	// Check if output is a terminal
	if stat, err := os.Stdout.Stat(); err == nil {
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			// Not a terminal, disable colors
			colorGreen = ""
			colorRed = ""
			colorYellow = ""
			colorReset = ""
		}
	}
}

// printSuccess prints a success message
func printSuccess(msg string) {
	fmt.Printf("%-70s%s[OK]%s\n", msg, colorGreen, colorReset)
}

// printError prints an error marker
func printError(msg string) {
	fmt.Printf("%s[FAIL]%s\n", colorRed, colorReset)
}

// printWarn prints a warning message
func printWarn(msg string) {
	fmt.Printf("%-70s%s[WARN]%s\n", msg, colorYellow, colorReset)
}
