package main

import (
	"bytes"
	"strings"
	"testing"
	"time"

	digest "github.com/opencontainers/go-digest"
	"github.com/stretchr/testify/assert"
	"go.podman.io/image/v5/types"
)

func TestProgressConsumerDone(t *testing.T) {
	ch := make(chan types.ProgressProperties, 8)
	var out bytes.Buffer

	fakeDigest := digest.FromString("test-blob")
	fakeSize := int64(1024 * 1024 * 10) // 10 MB

	ch <- types.ProgressProperties{
		Event:    types.ProgressEventNewArtifact,
		Artifact: types.BlobInfo{Digest: fakeDigest, Size: fakeSize},
	}
	ch <- types.ProgressProperties{
		Event: types.ProgressEventRead,
		Artifact: types.BlobInfo{
			Digest: fakeDigest,
			Size:   fakeSize,
		},
		Offset:       uint64(fakeSize / 2),
		OffsetUpdate: uint64(fakeSize / 2),
	}
	ch <- types.ProgressProperties{
		Event: types.ProgressEventDone,
		Artifact: types.BlobInfo{
			Digest: fakeDigest,
			Size:   fakeSize,
		},
		Offset: uint64(fakeSize),
	}
	close(ch) // simulate close on copy finish

	runProgressConsumer(ch, 30*time.Second, &out)

	output := out.String()

	assert.Contains(t, output, "done")
	assert.Contains(t, output, fakeDigest.Encoded()[:12])
	assert.Equal(t, 2, strings.Count(output, "\n"), "expected 2 lines: one read, one Done")
	assert.Contains(t, output, "5MiB", "progress line should show current offset, not total")
}

func TestProgressConsumerSkipped(t *testing.T) {
	ch := make(chan types.ProgressProperties, 4)
	var out bytes.Buffer

	fakeDigest := digest.FromString("cached-blob")

	ch <- types.ProgressProperties{
		Event: types.ProgressEventNewArtifact,
		Artifact: types.BlobInfo{
			Digest: fakeDigest,
			Size:   1024,
		},
	}
	ch <- types.ProgressProperties{
		Event:    types.ProgressEventSkipped,
		Artifact: types.BlobInfo{Digest: fakeDigest},
	}
	close(ch)

	runProgressConsumer(ch, 30*time.Second, &out)

	assert.Empty(t, out.String(), "skipped blobs should produce no output")
}

func TestProgressConsumerUnknownSize(t *testing.T) {
	ch := make(chan types.ProgressProperties, 4)
	var out bytes.Buffer

	fakeDigest := digest.FromString("mystery-blob")

	ch <- types.ProgressProperties{
		Event: types.ProgressEventNewArtifact,
		Artifact: types.BlobInfo{
			Digest: fakeDigest,
			Size:   -1,
		},
	}
	ch <- types.ProgressProperties{
		Event: types.ProgressEventRead,
		Artifact: types.BlobInfo{
			Digest: fakeDigest,
			Size:   -1,
		},
		Offset:       512,
		OffsetUpdate: 512,
	}
	close(ch)

	runProgressConsumer(ch, 30*time.Second, &out)

	assert.Contains(t, out.String(), "?", "unknown size should render as '?'")
}
