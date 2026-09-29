package main

import (
	"context"
	"flag"
	"fmt"
	"os"
	"time"

	"xcastphone/internal/session"
)

const version = "1.0.0"

func main() {
	var (
		showVersion = flag.Bool("version", false, "Print version information and exit")
		showHelp    = flag.Bool("help", false, "Show help message")
		verbose     = flag.Bool("verbose", false, "Enable verbose logging for debugging")
		debug       = flag.Bool("debug", false, "Enable detailed debug diagnostics and stream stats")
		testStream  = flag.Bool("test-stream", false, "Test Android capture and transport independently (headless)")
		bitrateMbps = flag.Int("bitrate", 8, "Video streaming bitrate in Mbps (default 8)")
		fps         = flag.Int("fps", 60, "Target frame rate (default 60)")
		maxSize     = flag.Int("max-size", 2400, "Maximum screen dimension in pixels (default 2400)")
		timeoutSec  = flag.Int("timeout", 90, "Pairing timeout in seconds (default 90)")
	)

	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "XCastPhone v%s - Terminal-First Android Screen-Casting\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: xcast [options]\n\n")
		fmt.Fprintf(os.Stderr, "Options:\n")
		flag.PrintDefaults()
		fmt.Fprintf(os.Stderr, "\nQuick Start:\n")
		fmt.Fprintf(os.Stderr, "  1. Run 'xcast'\n")
		fmt.Fprintf(os.Stderr, "  2. On Android phone: Settings > Developer options > Wireless debugging > 'Pair device with QR code'\n")
		fmt.Fprintf(os.Stderr, "  3. Scan the QR code displayed in the window to begin live mirroring\n")
		fmt.Fprintf(os.Stderr, "     (Do not use the standard camera app; use Wireless Debugging QR scanner)\n")
	}

	flag.Parse()

	if *showHelp {
		flag.Usage()
		os.Exit(0)
	}

	if *showVersion {
		fmt.Printf("xcast v%s\n", version)
		os.Exit(0)
	}

	cfg := session.Config{
		Verbose:      *verbose || *debug,
		Debug:        *debug,
		TestStream:   *testStream,
		Bitrate:      *bitrateMbps * 1000 * 1000,
		MaxDimension: *maxSize,
		TargetFPS:    *fps,
		PairTimeout:  time.Duration(*timeoutSec) * time.Second,
	}

	coordinator := session.NewCoordinator(cfg)

	ctx := context.Background()
	if err := coordinator.Run(ctx); err != nil {
		fmt.Fprintf(os.Stderr, "\nError: %v\n", err)
		os.Exit(1)
	}
}
