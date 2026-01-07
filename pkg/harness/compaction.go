package harness

import (
	"context"
	"fmt"
	"strings"

	"github.com/firebase/genkit/go/ai"
	"github.com/firebase/genkit/go/genkit"
)

type Compactor struct {
	genkitApp     *genkit.Genkit
	modelName     string
	config        CompactionConfig
	tokenCounter  TokenCounter
	contextWindow int
}

type TokenCounter interface {
	CountTokens(messages []*ai.Message) (int, error)
}

type SimpleTokenCounter struct{}

func (s *SimpleTokenCounter) CountTokens(messages []*ai.Message) (int, error) {
	total := 0
	for _, msg := range messages {
		for _, part := range msg.Content {
			if part.IsText() {
				total += len(strings.Fields(part.Text)) * 4 / 3
			}
		}
	}
	return total, nil
}

func NewCompactor(genkitApp *genkit.Genkit, modelName string, config CompactionConfig, contextWindow int) *Compactor {
	return &Compactor{
		genkitApp:     genkitApp,
		modelName:     modelName,
		config:        config,
		tokenCounter:  &SimpleTokenCounter{},
		contextWindow: contextWindow,
	}
}

func (c *Compactor) WithTokenCounter(counter TokenCounter) *Compactor {
	c.tokenCounter = counter
	return c
}

func (c *Compactor) CompactIfNeeded(ctx context.Context, history []*ai.Message) ([]*ai.Message, bool, error) {
	if !c.config.Enabled {
		return history, false, nil
	}

	currentTokens, err := c.tokenCounter.CountTokens(history)
	if err != nil {
		return history, false, fmt.Errorf("failed to count tokens: %w", err)
	}

	threshold := int(float64(c.contextWindow) * c.config.Threshold)
	if currentTokens < threshold {
		return history, false, nil
	}

	compacted, err := c.compact(ctx, history)
	if err != nil {
		return history, false, fmt.Errorf("compaction failed: %w", err)
	}

	return compacted, true, nil
}

func (c *Compactor) compact(ctx context.Context, history []*ai.Message) ([]*ai.Message, error) {
	if len(history) <= 2 {
		return history, nil
	}

	protectedTokens := c.config.ProtectTokens
	if protectedTokens <= 0 {
		protectedTokens = 40000
	}

	currentTokens, _ := c.tokenCounter.CountTokens(history)
	tokensToPreserve := 0
	preserveFromIdx := len(history)

	for i := len(history) - 1; i >= 0; i-- {
		msgTokens, _ := c.tokenCounter.CountTokens([]*ai.Message{history[i]})
		if tokensToPreserve+msgTokens > protectedTokens {
			preserveFromIdx = i + 1
			break
		}
		tokensToPreserve += msgTokens
	}

	if preserveFromIdx <= 1 {
		return history, nil
	}

	toSummarize := history[1:preserveFromIdx]
	if len(toSummarize) == 0 {
		return history, nil
	}

	summary, err := c.summarizeHistory(ctx, toSummarize)
	if err != nil {
		return history, err
	}

	compacted := make([]*ai.Message, 0, 2+len(history)-preserveFromIdx)

	if len(history) > 0 {
		compacted = append(compacted, history[0])
	}

	compacted = append(compacted, ai.NewUserTextMessage(fmt.Sprintf(
		"[CONTEXT COMPACTION: The following is a summary of %d previous messages totaling ~%d tokens]\n\n%s\n\n[END SUMMARY - Continuing from recent context]",
		len(toSummarize),
		currentTokens-tokensToPreserve,
		summary,
	)))

	compacted = append(compacted, history[preserveFromIdx:]...)

	return compacted, nil
}

func (c *Compactor) summarizeHistory(ctx context.Context, messages []*ai.Message) (string, error) {
	if c.genkitApp == nil {
		return c.fallbackSummarize(messages), nil
	}

	var historyText strings.Builder
	for i, msg := range messages {
		historyText.WriteString(fmt.Sprintf("\n--- Message %d (%s) ---\n", i+1, msg.Role))
		for _, part := range msg.Content {
			if part.IsText() {
				historyText.WriteString(part.Text)
				historyText.WriteString("\n")
			} else if part.IsToolRequest() {
				historyText.WriteString(fmt.Sprintf("[Tool Call: %s]\n", part.ToolRequest.Name))
			} else if part.IsToolResponse() {
				historyText.WriteString(fmt.Sprintf("[Tool Response: %s]\n", part.ToolResponse.Name))
			}
		}
	}

	prompt := fmt.Sprintf(`Summarize the following conversation history concisely, preserving:
1. Key decisions made
2. Important information discovered
3. Tool calls and their outcomes
4. Any errors encountered and their resolutions

Be concise but comprehensive. Focus on information that would be needed to continue the task.

CONVERSATION HISTORY:
%s

SUMMARY:`, historyText.String())

	resp, err := genkit.Generate(ctx, c.genkitApp,
		ai.WithModelName(c.modelName),
		ai.WithPrompt(prompt),
	)
	if err != nil {
		return c.fallbackSummarize(messages), nil
	}

	return resp.Text(), nil
}

func (c *Compactor) fallbackSummarize(messages []*ai.Message) string {
	var summary strings.Builder
	summary.WriteString("Previous conversation summary:\n")

	toolCalls := make(map[string]int)
	for _, msg := range messages {
		for _, part := range msg.Content {
			if part.IsToolRequest() {
				toolCalls[part.ToolRequest.Name]++
			}
		}
	}

	if len(toolCalls) > 0 {
		summary.WriteString("\nTools used:\n")
		for tool, count := range toolCalls {
			summary.WriteString(fmt.Sprintf("- %s: %d times\n", tool, count))
		}
	}

	summary.WriteString(fmt.Sprintf("\n[%d messages compacted]\n", len(messages)))
	return summary.String()
}

func (c *Compactor) ShouldCompact(history []*ai.Message) (bool, int, error) {
	if !c.config.Enabled {
		return false, 0, nil
	}

	currentTokens, err := c.tokenCounter.CountTokens(history)
	if err != nil {
		return false, 0, err
	}

	threshold := int(float64(c.contextWindow) * c.config.Threshold)
	return currentTokens >= threshold, currentTokens, nil
}
