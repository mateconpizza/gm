package txt

import (
	"strings"

	"github.com/mateconpizza/gm/pkg/ansi"
)

const (
	addMarker = "+\u00A0"
	delMarker = "-\u00A0"
)

// Diff Take two []byte and return a string with the complete diff.
func Diff(a, b []byte) string {
	return newDiff(a, b, addMarker, delMarker)
}

// DiffColorize colorizes the diff output.
func DiffColorize(text string) string {
	return newDiffColor(text, addMarker, delMarker)
}

func newDiff(a, b []byte, add, del string) string {
	linesA := strings.Split(string(a), "\n")
	linesB := strings.Split(string(b), "\n")
	m, n := len(linesA), len(linesB)

	// create the matrix for LCS (Longest Common Subsequence).
	dp := make([][]int, m+1)
	for i := range dp {
		dp[i] = make([]int, n+1)
	}

	// fill the DP (Dynamic Programming) matrix with the length of the LCS.
	// dp[i+1][j+1] stores the length of the LCS between linesA[:i+1] and linesB[:j+1].
	for i := range m {
		for j := range n {
			if linesA[i] == linesB[j] {
				// if lines match, LCS length increases by 1.
				dp[i+1][j+1] = dp[i][j] + 1
			} else {
				// otherwise, take the maximum value from the previous row or column.
				dp[i+1][j+1] = max(dp[i+1][j], dp[i][j+1])
			}
		}
	}

	// backtrack to construct the diff output.
	var diffLines []string

	i, j := m, n
	for i > 0 || j > 0 {
		switch {
		case i > 0 && j > 0 && linesA[i-1] == linesB[j-1]:
			// unchanged (common line)
			diffLines = append([]string{linesA[i-1]}, diffLines...)
			i--
			j--
		case j > 0 && (i == 0 || dp[i][j-1] >= dp[i-1][j]):
			// added
			diffLines = append([]string{add + linesB[j-1]}, diffLines...)
			j--
		case i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]):
			// deleted
			diffLines = append([]string{del + linesA[i-1]}, diffLines...)
			i--
		}
	}

	return strings.Join(diffLines, "\n")
}

func newDiffColor(text, add, del string) string {
	p := ansi.NewPalette()
	var r []string

	for l := range strings.SplitSeq(text, "\n") {
		switch {
		case strings.HasPrefix(l, add):
			r = append(r, " "+p.BrightGreen.Sprint(l))
		case strings.HasPrefix(l, del):
			r = append(r, " "+p.BrightRed.Sprint(l))
		default:
			r = append(r, " "+p.Dim.Sprint(l))
		}
	}

	return strings.Join(r, "\n")
}

// ExtractBlock extracts a block of text starting from the first line
// that has the startMarker until either the endMarker is found or EOF.
// If endMarker is empty, it extracts until EOF.
// Leading/trailing blank lines are removed.
func ExtractBlock(content []string, startMarker, endMarker string) string {
	return extractBlock(content, startMarker, endMarker, true)
}

// ExtractBlockRaw extracts a block of text starting from the first line
// that has the startMarker until either the endMarker is found or EOF.
// If endMarker is empty, it extracts until EOF.
func ExtractBlockRaw(content []string, startMarker, endMarker string) string {
	return extractBlock(content, startMarker, endMarker, false)
}

func extractBlock(content []string, startMarker, endMarker string, trim bool) string {
	block := make([]string, 0, len(content))
	inBlock := false

	for _, line := range content {
		if !inBlock {
			if strings.HasPrefix(line, startMarker) {
				inBlock = true
			}
			continue
		}

		if endMarker != "" && strings.HasPrefix(line, endMarker) {
			break
		}

		block = append(block, line)
	}

	if trim {
		start := 0
		for start < len(block) && strings.TrimSpace(block[start]) == "" {
			start++
		}

		end := len(block)
		for end > start && strings.TrimSpace(block[end-1]) == "" {
			end--
		}

		if start >= end {
			return ""
		}

		block = block[start:end]
	}

	if len(block) == 0 {
		return ""
	}

	return strings.Join(block, "\n")
}

func ExtractBlockBytes(d []byte, startMarker, endMarker string) []byte {
	lines := strings.Split(string(d), "\n")
	return []byte(ExtractBlock(lines, startMarker, endMarker))
}

func ExtractBlockBytesRaw(d []byte, startMarker, endMarker string) []byte {
	lines := strings.Split(string(d), "\n")
	return []byte(ExtractBlockRaw(lines, startMarker, endMarker))
}
