package safety

import (
	"bufio"
	"fmt"
	"os"
	"strings"
)

type DangerousPattern struct {
	Pattern     string
	Severity    string
	Description string
}

var dangerousPatterns = []DangerousPattern{
	{
		Pattern:     ":(){ :|:& };:",
		Severity:    "critical",
		Description: "Fork bomb - system resource attack",
	},
	{
		Pattern:     ":(){ : | : & }; :",
		Severity:    "critical",
		Description: "Fork bomb - system resource attack",
	},
	{
		Pattern:     "rm -rf /etc",
		Severity:    "critical",
		Description: "Removes system configuration directory",
	},
	{
		Pattern:     "rm -rf /usr",
		Severity:    "critical",
		Description: "Removes system user programs",
	},
	{
		Pattern:     "rm -rf /var",
		Severity:    "critical",
		Description: "Removes system variable data",
	},
	{
		Pattern:     "rm -rf /lib",
		Severity:    "critical",
		Description: "Removes system libraries",
	},
	{
		Pattern:     "rm -rf /",
		Severity:    "critical",
		Description: "Removes entire filesystem",
	},
	{
		Pattern:     "rm -rf /*",
		Severity:    "critical",
		Description: "Removes all root-level directories",
	},
	{
		Pattern:     "dd if=/dev/zero of=/",
		Severity:    "critical",
		Description: "Overwrites filesystem with zeros",
	},
	{
		Pattern:     "mkfs",
		Severity:    "critical",
		Description: "Creates new filesystem (data loss)",
	},
	{
		Pattern:     "sudo chmod 777 /",
		Severity:    "warning",
		Description: "Makes root directory world-writable",
	},
	{
		Pattern:     "wget http",
		Severity:    "info",
		Description: "Downloads file from internet (verify source)",
	},
	{
		Pattern:     "curl http",
		Severity:    "info",
		Description: "Downloads file from internet (verify source)",
	},
}

// SearchResult represents a line where a dangerous pattern was found
type SearchResult struct {
	LineNumber int
	Line       string
	Pattern    DangerousPattern
}

// isSafeContext checks if a pattern is in a safe context (like a comment)
func isSafeContext(line, pattern string) bool {
	// Find the position of the pattern in the line
	idx := strings.Index(line, pattern)
	if idx == -1 {
		return false
	}

	// Check if it's in a comment
	commentIdx := strings.Index(line, "#")
	if commentIdx != -1 && commentIdx < idx {
		// Pattern appears after a comment marker on the same line
		return true
	}

	return false
}

// ScanFileForDangerousPatterns scans a file for dangerous patterns and returns results
func ScanFileForDangerousPatterns(filename string) ([]SearchResult, error) {
	file, err := os.Open(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	var results []SearchResult
	scanner := bufio.NewScanner(file)
	lineNum := 0

	for scanner.Scan() {
		lineNum++
		line := scanner.Text()
		lineMatched := make(map[int]bool) // Track which patterns we've already matched on this line

		for patternIdx, pattern := range dangerousPatterns {
			if lineMatched[patternIdx] {
				continue // Already matched this pattern on this line
			}

			if strings.Contains(line, pattern.Pattern) {
				// Skip if it's in a comment
				if isSafeContext(line, pattern.Pattern) {
					continue
				}

				// Mark as matched and add result
				lineMatched[patternIdx] = true

				// Check if a more specific pattern already matched this line
				isMoreSpecific := false
				for i, result := range results {
					if result.LineNumber == lineNum {
						// If a more specific pattern (shorter != "rm -rf /") already matched, skip this one
						if len(result.Pattern.Pattern) > len(pattern.Pattern) {
							isMoreSpecific = true
							break
						}
						// If this pattern is more specific, replace the previous result
						if len(pattern.Pattern) > len(result.Pattern.Pattern) {
							results[i] = SearchResult{
								LineNumber: lineNum,
								Line:       line,
								Pattern:    pattern,
							}
							isMoreSpecific = true
							break
						}
					}
				}

				if !isMoreSpecific {
					results = append(results, SearchResult{
						LineNumber: lineNum,
						Line:       line,
						Pattern:    pattern,
					})
				}
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error scanning file: %w", err)
	}

	return results, nil
}

// FormatResultWithContext formats search results like grep -C2 (2 lines of context)
func FormatResultWithContext(filename string, result SearchResult, contextLines int) (string, error) {
	file, err := os.Open(filename)
	if err != nil {
		return "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()

	scanner := bufio.NewScanner(file)
	var allLines []string
	for scanner.Scan() {
		allLines = append(allLines, scanner.Text())
	}

	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("error scanning file: %w", err)
	}

	startLine := result.LineNumber - contextLines - 1 // -1 for 0-indexing
	if startLine < 0 {
		startLine = 0
	}
	endLine := result.LineNumber + contextLines
	if endLine > len(allLines) {
		endLine = len(allLines)
	}

	var output strings.Builder
	output.WriteString(fmt.Sprintf("[%s] %s\n", result.Pattern.Severity, result.Pattern.Description))
	output.WriteString(fmt.Sprintf("%s:%d: %s\n\n", filename, result.LineNumber, result.Line))

	for i := startLine; i < endLine; i++ {
		lineNum := i + 1
		prefix := "  "
		if lineNum == result.LineNumber {
			prefix = "> " // Mark the dangerous line
		}
		output.WriteString(fmt.Sprintf("%s%4d: %s\n", prefix, lineNum, allLines[i]))
	}

	return output.String(), nil
}
