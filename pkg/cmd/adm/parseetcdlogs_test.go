package adm

import (
	"bytes"
	"errors"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// failingWriter rejects every write, simulating a closed or broken stdout.
type failingWriter struct{ err error }

func (w failingWriter) Write([]byte) (int, error) { return 0, w.err }

func TestPrintTableOutput(t *testing.T) {
	var buf bytes.Buffer
	table := [][]string{{"ID", "COUNT"}, {"2024-01-01", "10"}, {"all", "1234567"}}

	require.NoError(t, printTable(&buf, table))
	// Golden output produced by the previous implementation, which wrote
	// directly to os.Stdout; the rendered layout must stay the same.
	assert.Equal(t, "ID\t\t\tCOUNT\t\n2024-01-01\t10\t\t\nall\t\t\t1234567\t\n", buf.String())
}

func TestPrintTableReturnsWriteError(t *testing.T) {
	errWrite := errors.New("write failed")

	err := printTable(failingWriter{err: errWrite}, [][]string{{"ID", "COUNT"}, {"all", "1"}})
	assert.ErrorIs(t, err, errWrite)
}
