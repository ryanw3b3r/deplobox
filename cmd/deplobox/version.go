package main

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"
)

var (
	// These will be set during build with -ldflags
	gitCommit = "unknown"
	buildDate = "unknown"
)

var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, build information, and runtime details for deplobox.`,
	Run:   runVersion,
}

func runVersion(cmd *cobra.Command, args []string) {
	fmt.Printf("deplobox version %s\n", version)
	fmt.Printf("  Git commit:  %s\n", gitCommit)
	fmt.Printf("  Build date:  %s\n", buildDate)
	fmt.Printf("  Go version:  %s\n", runtime.Version())
	fmt.Printf("  OS/Arch:     %s/%s\n", runtime.GOOS, runtime.GOARCH)
}
