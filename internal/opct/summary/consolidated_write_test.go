package summary

import (
	"errors"
	"os"
	"path/filepath"
	"syscall"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWriteFileTestListPersistsData(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "tests.txt")

	require.NoError(t, writeFileTestList(filename, []string{"test-a", "test-b"}))
	// files are opened in append mode; keep that behavior.
	require.NoError(t, writeFileTestList(filename, []string{"test-c"}))

	got, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, "test-a\ntest-b\ntest-c\n", string(got))
}

func TestWriteErrorToFilePersistsData(t *testing.T) {
	filename := filepath.Join(t.TempDir(), "failure.txt")

	require.NoError(t, writeErrorToFile(filename, "failure details"))

	got, err := os.ReadFile(filename)
	require.NoError(t, err)
	assert.Equal(t, "failure details", string(got))
}

// /dev/full accepts open but fails every write with ENOSPC. The data is
// buffered, so the failure only surfaces when the buffer is flushed; it was
// silently dropped when the flush was deferred without checking its error.
//
// The writers exit the process when the open fails, so skip unless the
// device can actually be opened for writing.
func requireDevFull(t *testing.T) string {
	t.Helper()
	const devFull = "/dev/full"
	f, err := os.OpenFile(devFull, os.O_WRONLY, 0)
	if err != nil {
		t.Skipf("%s not writable: %v", devFull, err)
	}
	require.NoError(t, f.Close())
	return devFull
}

func TestWriteFileTestListReturnsFlushError(t *testing.T) {
	devFull := requireDevFull(t)

	err := writeFileTestList(devFull, []string{"test-a"})
	assert.ErrorIs(t, err, syscall.ENOSPC)
}

func TestWriteErrorToFileReturnsFlushError(t *testing.T) {
	devFull := requireDevFull(t)

	err := writeErrorToFile(devFull, "failure details")
	assert.ErrorIs(t, err, syscall.ENOSPC)
}

func TestCloseWrittenFile(t *testing.T) {
	// An already-closed file makes Close fail deterministically.
	closedFile := func(t *testing.T) *os.File {
		t.Helper()
		fd, err := os.Create(filepath.Join(t.TempDir(), "closed.txt"))
		require.NoError(t, err)
		require.NoError(t, fd.Close())
		return fd
	}

	t.Run("reports close error when no earlier error", func(t *testing.T) {
		var err error
		closeWrittenFile(closedFile(t), &err)
		assert.ErrorIs(t, err, os.ErrClosed)
	})

	t.Run("keeps earlier error", func(t *testing.T) {
		errEarlier := errors.New("write failed")
		err := errEarlier
		closeWrittenFile(closedFile(t), &err)
		assert.Same(t, errEarlier, err)
	})

	t.Run("no error on successful close", func(t *testing.T) {
		fd, err := os.Create(filepath.Join(t.TempDir(), "open.txt"))
		require.NoError(t, err)
		closeWrittenFile(fd, &err)
		assert.NoError(t, err)
	})
}
