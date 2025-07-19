package llm

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/bedrockruntime"
	"github.com/sirupsen/logrus"

	"github.com/jamesonstone/what-did-you-get-done-this-week/internal/models"
	pkgConfig "github.com/jamesonstone/what-did-you-get-done-this-week/pkg/config"
)

type Service struct {
	client *bedrockruntime.Client
	config *pkgConfig.Config
}

type WeeklySummary struct {
	Paragraph    string   `json:"paragraph"`
	BulletPoints []string `json:"bullet_points"`
	Model        string   `json:"model"`
	CostCents    int      `json:"cost_cents"`
}

type ClaudeRequest struct {
	AnthropicVersion string    `json:"anthropic_version"`
	MaxTokens        int       `json:"max_tokens"`
	Messages         []Message `json:"messages"`
}

type Message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type ClaudeResponse struct {
	Content []ContentBlock `json:"content"`
	Usage   Usage          `json:"usage"`
}

type ContentBlock struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

type Usage struct {
	InputTokens  int `json:"input_tokens"`
	OutputTokens int `json:"output_tokens"`
}

func NewService(cfg *pkgConfig.Config) (*Service, error) {
	awsCfg, err := config.LoadDefaultConfig(context.TODO(),
		config.WithRegion(cfg.AWSRegion))
	if err != nil {
		return nil, fmt.Errorf("failed to load AWS config: %w", err)
	}

	return &Service{
		client: bedrockruntime.NewFromConfig(awsCfg),
		config: cfg,
	}, nil
}

func (s *Service) GenerateWeeklySummary(ctx context.Context, entries []*models.Entry) (*WeeklySummary, error) {
	prompt := s.buildWeeklySummaryPrompt(entries)
	
	logrus.WithFields(logrus.Fields{
		"entries_count": len(entries),
		"model":         s.config.LLMModel,
	}).Info("Generating weekly summary")

	response, err := s.callClaude(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("failed to call Claude: %w", err)
	}

	summary, err := s.parseWeeklySummaryResponse(response)
	if err != nil {
		return nil, fmt.Errorf("failed to parse summary response: %w", err)
	}

	summary.Model = s.config.LLMModel
	summary.CostCents = s.estimateCost(response.Usage)

	logrus.WithFields(logrus.Fields{
		"input_tokens":  response.Usage.InputTokens,
		"output_tokens": response.Usage.OutputTokens,
		"cost_cents":    summary.CostCents,
	}).Info("Weekly summary generated")

	return summary, nil
}

func (s *Service) buildWeeklySummaryPrompt(entries []*models.Entry) string {
	var entriesText strings.Builder
	
	days := []string{"Monday", "Tuesday", "Wednesday", "Thursday", "Friday"}
	
	for i, entry := range entries {
		if i < len(days) {
			entriesText.WriteString(fmt.Sprintf("%s: %s\n", days[i], entry.RawContent))
		}
	}

	return fmt.Sprintf(`System: You are tasked with summarizing a user's weekly accomplishments in the tone and style of Elon Musk - direct, output-driven, and focused on execution. Create a concise summary paragraph followed by 3-5 key bullet points of the most important achievements.

The summary should:
- Be written in Elon's assertive, no-nonsense tone
- Focus on tangible outputs and results
- Highlight the most impactful work
- Be motivational but realistic
- Avoid fluff or unnecessary praise

User's weekly entries:
%s

Please respond with:
1. A single paragraph summary (2-3 sentences)
2. 3-5 bullet points of key accomplishments

Format your response as:
SUMMARY: [paragraph here]
BULLETS:
• [bullet 1]
• [bullet 2]
• [bullet 3]
etc.`, entriesText.String())
}

func (s *Service) callClaude(ctx context.Context, prompt string) (*ClaudeResponse, error) {
	request := ClaudeRequest{
		AnthropicVersion: "bedrock-2023-05-31",
		MaxTokens:        1000,
		Messages: []Message{
			{
				Role:    "user",
				Content: prompt,
			},
		},
	}

	requestBody, err := json.Marshal(request)
	if err != nil {
		return nil, fmt.Errorf("failed to marshal request: %w", err)
	}

	input := &bedrockruntime.InvokeModelInput{
		ModelId:     aws.String(s.config.LLMModel),
		ContentType: aws.String("application/json"),
		Body:        requestBody,
	}

	result, err := s.client.InvokeModel(ctx, input)
	if err != nil {
		return nil, fmt.Errorf("failed to invoke model: %w", err)
	}

	var response ClaudeResponse
	if err := json.Unmarshal(result.Body, &response); err != nil {
		return nil, fmt.Errorf("failed to unmarshal response: %w", err)
	}

	return &response, nil
}

func (s *Service) parseWeeklySummaryResponse(response *ClaudeResponse) (*WeeklySummary, error) {
	if len(response.Content) == 0 {
		return nil, fmt.Errorf("no content in response")
	}

	text := response.Content[0].Text
	
	// Parse the structured response
	lines := strings.Split(text, "\n")
	var summary string
	var bullets []string
	inBullets := false

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		if strings.HasPrefix(strings.ToUpper(line), "SUMMARY:") {
			summary = strings.TrimSpace(strings.TrimPrefix(line, "SUMMARY:"))
			summary = strings.TrimSpace(strings.TrimPrefix(summary, "summary:"))
		} else if strings.ToUpper(line) == "BULLETS:" {
			inBullets = true
		} else if inBullets && strings.HasPrefix(line, "•") {
			bullet := strings.TrimSpace(strings.TrimPrefix(line, "•"))
			if bullet != "" {
				bullets = append(bullets, bullet)
			}
		} else if inBullets && strings.HasPrefix(line, "-") {
			bullet := strings.TrimSpace(strings.TrimPrefix(line, "-"))
			if bullet != "" {
				bullets = append(bullets, bullet)
			}
		}
	}

	// Fallback parsing if structured format wasn't followed
	if summary == "" || len(bullets) == 0 {
		return s.fallbackParse(text)
	}

	return &WeeklySummary{
		Paragraph:    summary,
		BulletPoints: bullets,
	}, nil
}

func (s *Service) fallbackParse(text string) (*WeeklySummary, error) {
	// Simple fallback: first paragraph as summary, bullet points as-is
	paragraphs := strings.Split(text, "\n\n")
	
	var summary string
	var bullets []string
	
	if len(paragraphs) > 0 {
		summary = strings.TrimSpace(paragraphs[0])
	}
	
	// Look for bullet points in any paragraph
	for _, para := range paragraphs {
		lines := strings.Split(para, "\n")
		for _, line := range lines {
			line = strings.TrimSpace(line)
			if strings.HasPrefix(line, "•") || strings.HasPrefix(line, "-") {
				bullet := strings.TrimSpace(strings.TrimPrefix(line, "•"))
				bullet = strings.TrimSpace(strings.TrimPrefix(bullet, "-"))
				if bullet != "" {
					bullets = append(bullets, bullet)
				}
			}
		}
	}
	
	// If no bullets found, create some from the summary
	if len(bullets) == 0 {
		bullets = []string{summary}
	}
	
	return &WeeklySummary{
		Paragraph:    summary,
		BulletPoints: bullets,
	}, nil
}

func (s *Service) estimateCost(usage Usage) int {
	// Rough cost estimation for Claude Haiku (cheapest model)
	// Input: ~$0.25 per 1M tokens, Output: ~$1.25 per 1M tokens
	inputCostCents := (usage.InputTokens * 25) / 1000000  // $0.25 per 1M tokens
	outputCostCents := (usage.OutputTokens * 125) / 1000000 // $1.25 per 1M tokens
	
	totalCents := inputCostCents + outputCostCents
	if totalCents < 1 {
		return 1 // Minimum 1 cent
	}
	
	return totalCents
}