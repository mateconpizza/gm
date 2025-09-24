package txt

import (
	"strings"

	"github.com/mateconpizza/gm/internal/ui/color"
)

// Diff Take two []byte and return a string with the complete diff.
func Diff(a, b []byte) string {
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
			diffLines = append([]string{"+" + linesB[j-1]}, diffLines...)
			j--
		case i > 0 && (j == 0 || dp[i][j-1] < dp[i-1][j]):
			// deleted
			diffLines = append([]string{"-" + linesA[i-1]}, diffLines...)
			i--
		}
	}

	return strings.Join(diffLines, "\n")
}

// DiffColor colorizes the diff output.
func DiffColor(s string) string {
	var r []string

	for l := range strings.SplitSeq(s, "\n") {
		switch {
		case strings.HasPrefix(l, "+"):
			r = append(r, "  "+color.BrightGreen(l).String())
		case strings.HasPrefix(l, "-"):
			r = append(r, "  "+color.BrightRed(l).String())
		default:
			r = append(r, "  "+color.BrightGray(l).Italic().String())
		}
	}

	return strings.Join(r, "\n")
}

// ExtractBlock extracts a block of text starting from the first line
// that has the startMarker until either the endMarker is found or EOF.
// If endMarker is empty, it extracts until EOF.
func ExtractBlock(content []string, startMarker, endMarker string) string {
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

	// Trim leading/trailing blank lines
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

	return strings.Join(block[start:end], "\n")
}

func ExtractBlockBytes(d []byte, startMarker, endMarker string) []byte {
	lines := strings.Split(string(d), "\n")
	return []byte(ExtractBlock(lines, startMarker, endMarker))
}
