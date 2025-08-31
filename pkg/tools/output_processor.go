package tools

import (
	"fmt"
	"strings"
	"unicode/utf8"
)

// DefaultOutputProcessor implements intelligent output processing with context-aware truncation
type DefaultOutputProcessor struct {
	tokenEstimateRatio float64 // Characters per token ratio for estimation
}

// NewDefaultOutputProcessor creates a new default output processor
func NewDefaultOutputProcessor() *DefaultOutputProcessor {
	return &DefaultOutputProcessor{
		tokenEstimateRatio: 4.0, // Conservative estimate: 4 characters per token
	}
}

// ProcessOutput processes raw tool output based on content type and context constraints
func (p *DefaultOutputProcessor) ProcessOutput(output string, toolName string, maxTokens int, strategy TruncationStrategy) (*ProcessedOutput, error) {
	if output == "" {
		return &ProcessedOutput{
			Content:     "",
			TokenCount:  0,
			Truncated:   false,
			ContentType: "empty",
		}, nil
	}

	originalLength := len(output)
	estimatedTokens := p.EstimateTokens(output)
	contentType := p.detectContentType(output, toolName)

	// If output is within limits, return as-is
	if estimatedTokens <= maxTokens {
		return &ProcessedOutput{
			Content:        output,
			TokenCount:     estimatedTokens,
			Truncated:      false,
			OriginalLength: originalLength,
			ContentType:    contentType,
		}, nil
	}

	// Apply truncation strategy
	truncatedContent, processingNotes := p.applyTruncationStrategy(output, toolName, maxTokens, strategy, contentType)
	truncatedTokens := p.EstimateTokens(truncatedContent)

	return &ProcessedOutput{
		Content:         truncatedContent,
		TokenCount:      truncatedTokens,
		Truncated:       true,
		OriginalLength:  originalLength,
		ProcessingNotes: processingNotes,
		ContentType:     contentType,
	}, nil
}

// EstimateTokens estimates token count for given output using character ratio
func (p *DefaultOutputProcessor) EstimateTokens(output string) int {
	if output == "" {
		return 0
	}
	
	// Use character count with conservative ratio
	charCount := utf8.RuneCountInString(output)
	tokens := int(float64(charCount) / p.tokenEstimateRatio)
	
	// Minimum 1 token for non-empty content
	if tokens == 0 && len(output) > 0 {
		tokens = 1
	}
	
	return tokens
}

// SupportedContentTypes returns content types this processor can handle
func (p *DefaultOutputProcessor) SupportedContentTypes() []string {
	return []string{
		"text/plain",
		"application/json",
		"text/csv",
		"text/html",
		"text/xml",
		"application/xml",
		"text/markdown",
		"source_code",
		"log_output",
		"directory_listing",
		"search_results",
		"error_output",
	}
}

// detectContentType attempts to identify the content type based on structure and tool name
func (p *DefaultOutputProcessor) detectContentType(output string, toolName string) string {
	toolName = strings.ToLower(toolName)
	output = strings.TrimSpace(output)

	// Tool-based content type detection
	switch {
	case strings.Contains(toolName, "list_dir") || strings.Contains(toolName, "directory"):
		return "directory_listing"
	case strings.Contains(toolName, "search") || strings.Contains(toolName, "grep"):
		return "search_results"
	case strings.Contains(toolName, "error") || strings.HasPrefix(output, "Error:"):
		return "error_output"
	case strings.Contains(toolName, "log"):
		return "log_output"
	}

	// Content-based detection
	switch {
	case p.isJSON(output):
		return "application/json"
	case p.isXML(output):
		return "application/xml"
	case p.isHTML(output):
		return "text/html"
	case p.isCSV(output):
		return "text/csv"
	case p.isMarkdown(output):
		return "text/markdown"
	case p.isSourceCode(output):
		return "source_code"
	default:
		return "text/plain"
	}
}

// applyTruncationStrategy applies the specified truncation strategy
func (p *DefaultOutputProcessor) applyTruncationStrategy(output string, toolName string, maxTokens int, strategy TruncationStrategy, contentType string) (string, string) {
	maxChars := int(float64(maxTokens) * p.tokenEstimateRatio)
	
	switch strategy {
	case TruncationStrategyNone:
		return output, "No truncation applied - content may exceed context limits"
		
	case TruncationStrategyHead:
		if len(output) <= maxChars {
			return output, ""
		}
		truncated := p.truncateAtWordBoundary(output[:maxChars])
		return truncated + "\n\n[... output truncated ...]", 
			fmt.Sprintf("Truncated to first %d characters", len(truncated))
			
	case TruncationStrategyTail:
		if len(output) <= maxChars {
			return output, ""
		}
		startPos := len(output) - maxChars
		if startPos < 0 {
			startPos = 0
		}
		truncated := p.truncateAtWordBoundary(output[startPos:])
		return "[... earlier output truncated ...]\n\n" + truncated,
			fmt.Sprintf("Truncated to last %d characters", len(truncated))
			
	case TruncationStrategyHeadTail:
		if len(output) <= maxChars {
			return output, ""
		}
		headChars := maxChars / 2
		tailChars := maxChars - headChars - 50 // Reserve space for truncation message
		
		head := p.truncateAtWordBoundary(output[:headChars])
		tail := p.truncateAtWordBoundary(output[len(output)-tailChars:])
		
		return head + "\n\n[... middle section truncated ...]\n\n" + tail,
			fmt.Sprintf("Truncated to first %d and last %d characters", len(head), len(tail))
			
	case TruncationStrategySummary:
		return p.applySummaryTruncation(output, contentType, maxChars)
		
	case TruncationStrategyIntelligent:
		return p.applyIntelligentTruncation(output, toolName, contentType, maxChars)
		
	default:
		// Fallback to head truncation
		return p.applyTruncationStrategy(output, toolName, maxTokens, TruncationStrategyHead, contentType)
	}
}

// applyIntelligentTruncation uses content-aware truncation based on content type
func (p *DefaultOutputProcessor) applyIntelligentTruncation(output string, toolName string, contentType string, maxChars int) (string, string) {
	if len(output) <= maxChars {
		return output, ""
	}

	switch contentType {
	case "application/json":
		return p.truncateJSON(output, maxChars)
	case "directory_listing":
		return p.truncateDirectoryListing(output, maxChars)
	case "search_results":
		return p.truncateSearchResults(output, maxChars)
	case "source_code":
		return p.truncateSourceCode(output, maxChars)
	case "log_output":
		return p.truncateLogOutput(output, maxChars)
	case "error_output":
		// For errors, prefer tail (most recent error information)
		return p.applyTruncationStrategy(output, toolName, maxChars/4, TruncationStrategyTail, contentType) // Convert back to token estimate
	default:
		// For plain text, use head-tail approach
		return p.applyTruncationStrategy(output, toolName, maxChars/4, TruncationStrategyHeadTail, contentType)
	}
}

// applySummaryTruncation creates a summary when possible
func (p *DefaultOutputProcessor) applySummaryTruncation(output string, contentType string, maxChars int) (string, string) {
	switch contentType {
	case "directory_listing":
		return p.summarizeDirectoryListing(output, maxChars)
	case "search_results":
		return p.summarizeSearchResults(output, maxChars)
	case "log_output":
		return p.summarizeLogOutput(output, maxChars)
	default:
		// Fallback to head truncation for content that can't be easily summarized
		truncated := p.truncateAtWordBoundary(output[:maxChars])
		return truncated + "\n\n[... content summarized ...]",
			fmt.Sprintf("Content summarized due to length (%d chars)", len(output))
	}
}

// Content type detection helpers
func (p *DefaultOutputProcessor) isJSON(output string) bool {
	trimmed := strings.TrimSpace(output)
	return (strings.HasPrefix(trimmed, "{") && strings.HasSuffix(trimmed, "}")) ||
		(strings.HasPrefix(trimmed, "[") && strings.HasSuffix(trimmed, "]"))
}

func (p *DefaultOutputProcessor) isXML(output string) bool {
	trimmed := strings.TrimSpace(output)
	return strings.HasPrefix(trimmed, "<") && strings.HasSuffix(trimmed, ">")
}

func (p *DefaultOutputProcessor) isHTML(output string) bool {
	return strings.Contains(strings.ToLower(output), "<html") || 
		strings.Contains(strings.ToLower(output), "<!doctype")
}

func (p *DefaultOutputProcessor) isCSV(output string) bool {
	lines := strings.Split(output, "\n")
	if len(lines) < 2 {
		return false
	}
	
	// Check if first few lines have consistent comma separation
	firstLineCommas := strings.Count(lines[0], ",")
	if firstLineCommas == 0 {
		return false
	}
	
	for i := 1; i < min(5, len(lines)); i++ {
		if strings.Count(lines[i], ",") != firstLineCommas {
			return false
		}
	}
	return true
}

func (p *DefaultOutputProcessor) isMarkdown(output string) bool {
	// Look for common markdown patterns
	patterns := []string{"# ", "## ", "* ", "- ", "```", "[", "]("}
	count := 0
	for _, pattern := range patterns {
		if strings.Contains(output, pattern) {
			count++
		}
	}
	return count >= 2 // At least 2 markdown patterns
}

func (p *DefaultOutputProcessor) isSourceCode(output string) bool {
	// Look for common code patterns
	codePatterns := []string{"{", "}", "function", "class", "def ", "import ", "#include", "var ", "const "}
	count := 0
	for _, pattern := range codePatterns {
		if strings.Contains(output, pattern) {
			count++
		}
	}
	return count >= 3 // At least 3 code-like patterns
}

// Content-specific truncation methods
func (p *DefaultOutputProcessor) truncateJSON(output string, maxChars int) (string, string) {
	if len(output) <= maxChars {
		return output, ""
	}
	
	// Try to truncate at a complete JSON object boundary
	truncated := output[:maxChars]
	
	// Find the last complete object/array
	lastBrace := strings.LastIndex(truncated, "}")
	lastBracket := strings.LastIndex(truncated, "]")
	cutPoint := max(lastBrace, lastBracket)
	
	if cutPoint > maxChars/2 { // Only use if we're keeping reasonable amount
		truncated = output[:cutPoint+1]
	}
	
	return truncated + "\n... [JSON truncated]",
		fmt.Sprintf("JSON truncated at object boundary (%d chars)", len(truncated))
}

func (p *DefaultOutputProcessor) truncateDirectoryListing(output string, maxChars int) (string, string) {
	lines := strings.Split(output, "\n")
	if len(lines) <= 10 {
		return output, "" // Short listings don't need truncation
	}
	
	// Keep first few and last few entries
	keepLines := maxChars / 20 // Estimate ~20 chars per line
	if keepLines > len(lines) {
		return output, ""
	}
	
	headLines := keepLines / 2
	tailLines := keepLines - headLines
	
	result := strings.Join(lines[:headLines], "\n") +
		fmt.Sprintf("\n... [%d entries omitted] ...\n", len(lines)-keepLines) +
		strings.Join(lines[len(lines)-tailLines:], "\n")
		
	return result, fmt.Sprintf("Directory listing truncated (%d entries shown, %d total)", keepLines, len(lines))
}

func (p *DefaultOutputProcessor) truncateSearchResults(output string, maxChars int) (string, string) {
	lines := strings.Split(output, "\n")
	
	// Keep the most relevant lines (usually first few results)
	keepLines := maxChars / 30 // Estimate ~30 chars per search result line
	if keepLines > len(lines) || keepLines < 5 {
		return p.truncateAtWordBoundary(output[:maxChars]) + "\n... [search results truncated]",
			fmt.Sprintf("Search results truncated (%d chars)", maxChars)
	}
	
	result := strings.Join(lines[:keepLines], "\n")
	return result + fmt.Sprintf("\n... [%d more search results] ...", len(lines)-keepLines),
		fmt.Sprintf("Search results truncated (%d results shown, %d total)", keepLines, len(lines))
}

func (p *DefaultOutputProcessor) truncateSourceCode(output string, maxChars int) (string, string) {
	// For source code, try to preserve structure
	lines := strings.Split(output, "\n")
	
	charCount := 0
	lineCount := 0
	
	for i, line := range lines {
		charCount += len(line) + 1 // +1 for newline
		if charCount > maxChars {
			result := strings.Join(lines[:i], "\n")
			return result + fmt.Sprintf("\n... [%d lines truncated] ...", len(lines)-i),
				fmt.Sprintf("Source code truncated (%d lines shown, %d total)", i, len(lines))
		}
		lineCount++
	}
	
	return output, ""
}

func (p *DefaultOutputProcessor) truncateLogOutput(output string, maxChars int) (string, string) {
	// For logs, prefer recent entries (tail)
	lines := strings.Split(output, "\n")
	
	charCount := 0
	for i := len(lines) - 1; i >= 0; i-- {
		charCount += len(lines[i]) + 1
		if charCount > maxChars {
			result := strings.Join(lines[i+1:], "\n")
			return fmt.Sprintf("... [%d log entries omitted] ...\n", i+1) + result,
				fmt.Sprintf("Log output truncated (%d entries shown, %d total)", len(lines)-(i+1), len(lines))
		}
	}
	
	return output, ""
}

// Summary methods for different content types
func (p *DefaultOutputProcessor) summarizeDirectoryListing(output string, maxChars int) (string, string) {
	lines := strings.Split(output, "\n")
	
	fileCount := 0
	dirCount := 0
	
	var importantFiles []string
	
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		
		// Simple heuristics for directory vs file
		if strings.HasSuffix(line, "/") {
			dirCount++
		} else {
			fileCount++
			// Collect important file types
			if p.isImportantFile(line) {
				importantFiles = append(importantFiles, line)
			}
		}
	}
	
	summary := fmt.Sprintf("Directory Summary: %d files, %d directories\n", fileCount, dirCount)
	if len(importantFiles) > 0 {
		summary += "Important files: " + strings.Join(importantFiles[:min(5, len(importantFiles))], ", ")
		if len(importantFiles) > 5 {
			summary += fmt.Sprintf(" and %d more", len(importantFiles)-5)
		}
	}
	
	return summary, fmt.Sprintf("Directory listing summarized (%d total entries)", len(lines))
}

func (p *DefaultOutputProcessor) summarizeSearchResults(output string, maxChars int) (string, string) {
	lines := strings.Split(output, "\n")
	matchCount := len(lines)
	
	// Show first few matches as examples
	keepLines := min(3, len(lines))
	examples := strings.Join(lines[:keepLines], "\n")
	
	summary := fmt.Sprintf("Search Results: %d matches found\n\nFirst %d matches:\n%s", 
		matchCount, keepLines, examples)
	
	if matchCount > keepLines {
		summary += fmt.Sprintf("\n... and %d more matches", matchCount-keepLines)
	}
	
	return summary, fmt.Sprintf("Search results summarized (%d total matches)", matchCount)
}

func (p *DefaultOutputProcessor) summarizeLogOutput(output string, maxChars int) (string, string) {
	lines := strings.Split(output, "\n")
	
	errorCount := 0
	warningCount := 0
	infoCount := 0
	
	var recentErrors []string
	
	for _, line := range lines {
		lower := strings.ToLower(line)
		switch {
		case strings.Contains(lower, "error"):
			errorCount++
			if len(recentErrors) < 3 {
				recentErrors = append(recentErrors, line)
			}
		case strings.Contains(lower, "warn"):
			warningCount++
		default:
			infoCount++
		}
	}
	
	summary := fmt.Sprintf("Log Summary: %d total entries (%d errors, %d warnings, %d info)\n",
		len(lines), errorCount, warningCount, infoCount)
	
	if len(recentErrors) > 0 {
		summary += "\nRecent errors:\n" + strings.Join(recentErrors, "\n")
	}
	
	return summary, fmt.Sprintf("Log output summarized (%d total entries)", len(lines))
}

// Utility methods
func (p *DefaultOutputProcessor) truncateAtWordBoundary(text string) string {
	if len(text) < 10 {
		return text
	}
	
	// Look for word boundary near the end
	lastSpace := strings.LastIndex(text[:len(text)-5], " ")
	if lastSpace > len(text)/2 {
		return text[:lastSpace]
	}
	
	return text
}

func (p *DefaultOutputProcessor) isImportantFile(filename string) bool {
	important := []string{".md", ".txt", ".json", ".yaml", ".yml", ".config", ".env", 
		".py", ".go", ".js", ".ts", ".java", ".cpp", ".h", "Dockerfile", "Makefile", "README"}
	
	filename = strings.ToLower(filename)
	for _, ext := range important {
		if strings.HasSuffix(filename, strings.ToLower(ext)) {
			return true
		}
	}
	return false
}

// Helper functions
func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}