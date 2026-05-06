package main

import (
	"fmt"
	"io"
	"os"
	"time"

	units "github.com/docker/go-units"
	"go.podman.io/image/v5/types"
	"golang.org/x/term"
)

func runProgressConsumer(ch <-chan types.ProgressProperties, interval time.Duration, out io.Writer) {
	type blobState struct {
		total int64
	}
	blobs := map[string]*blobState{}

	for props := range ch {
		dig := props.Artifact.Digest.Encoded()

		switch props.Event {
		case types.ProgressEventNewArtifact:
			blobs[dig] = &blobState{total: props.Artifact.Size}

		case types.ProgressEventRead:
			b := blobs[dig]
			bytesPerSec := float64(props.OffsetUpdate) / interval.Seconds()
			totalStr := "?"
			if b != nil && b.total > 0 {
				totalStr = units.BytesSize(float64(b.total))
			}
			// TODO: Make this prettier.
			fmt.Fprintf(out, "%s  %s  %s / %s  (%.1f MB/s)\n",
				time.Now().UTC().Format(time.RFC3339), dig[:12], units.BytesSize(float64(props.Offset)), totalStr, bytesPerSec/1e6)
		case types.ProgressEventDone:
			if b, ok := blobs[dig]; ok {
				fmt.Fprintf(out, "%s  %s  done (%s)\n",
					time.Now().UTC().Format(time.RFC3339),
					dig[:12],
					units.BytesSize(float64(b.total)),
				)
				delete(blobs, dig)
			}
		case types.ProgressEventSkipped:
			delete(blobs, dig)
		}
	}
}

func resolveProgressInterval(explicit time.Duration) time.Duration {
	if explicit != 0 {
		return explicit
	}
	if !term.IsTerminal(int(os.Stderr.Fd())) {
		return 30 * time.Second
	}
	return 0 // Suppress for TTY
}
