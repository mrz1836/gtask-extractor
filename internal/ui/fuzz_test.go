package ui

import (
	"io"
	"strings"
	"testing"

	"github.com/stretchr/testify/require"
)

// FuzzSelectIndex verifies that arbitrary input never makes SelectIndex panic
// and never yields an out-of-range selection. The caller indexes a slice with
// the returned value (lists[idx]), so an out-of-range index would be a panic in
// production — this pins the [0,n) invariant across any input.
func FuzzSelectIndex(f *testing.F) {
	f.Add("1\n", 3)
	f.Add("q\n", 5)
	f.Add("", 1)
	f.Add("99999999999999999999999999\n2\n", 4) // Atoi overflow, then a valid pick
	f.Add("  QUIT  \n", 2)
	f.Add("\x00\xff\nfoo\n-1\n0\n", 3)

	f.Fuzz(func(t *testing.T, input string, n int) {
		// SelectIndex is only meaningful for a positive option count; bound n so
		// the fuzzer explores the real range without absurd values.
		if n < 1 {
			n = 1
		}

		if n > 1000 {
			n = n%1000 + 1
		}

		idx, quit, err := SelectIndex(io.Discard, strings.NewReader(input), "pick: ", n)
		require.False(t, err == nil && !quit && (idx < 0 || idx >= n),
			"SelectIndex returned out-of-range index %d for n=%d (input=%q)", idx, n, input)
	})
}
