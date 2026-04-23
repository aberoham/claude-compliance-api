package compliance

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// Classification taxonomy constants adapted from "How People Use ChatGPT" (Chatterji et al., 2025).

// Intent categories describe the user's purpose in a message.
type Intent string

const (
	IntentAsking      Intent = "asking"
	IntentDoing       Intent = "doing"
	IntentExpressing  Intent = "expressing"
	IntentUnknown     Intent = "unknown"
)

// TopicFine represents the 24 fine-grained topic categories.
type TopicFine string

const (
	TopicHowToAdvice              TopicFine = "how_to_advice"
	TopicTutoringOrTeaching       TopicFine = "tutoring_or_teaching"
	TopicCreativeIdeation         TopicFine = "creative_ideation"
	TopicHealthFitnessBeauty      TopicFine = "health_fitness_beauty_or_self_care"
	TopicEditOrCritique           TopicFine = "edit_or_critique_provided_text"
	TopicPersonalWriting          TopicFine = "personal_writing_or_communication"
	TopicTranslation              TopicFine = "translation"
	TopicArgumentOrSummary        TopicFine = "argument_or_summary_generation"
	TopicWriteFiction             TopicFine = "write_fiction"
	TopicSpecificInfo             TopicFine = "specific_info"
	TopicPurchasableProducts      TopicFine = "purchasable_products"
	TopicCookingRecipes           TopicFine = "cooking_and_recipes"
	TopicComputerProgramming      TopicFine = "computer_programming"
	TopicMathematicalCalculation  TopicFine = "mathematical_calculation"
	TopicDataAnalysis             TopicFine = "data_analysis"
	TopicCreateImage              TopicFine = "create_an_image"
	TopicAnalyzeImage             TopicFine = "analyze_an_image"
	TopicOtherMedia               TopicFine = "generate_or_retrieve_other_media"
	TopicGreetingsChitchat        TopicFine = "greetings_and_chitchat"
	TopicRelationshipsReflection  TopicFine = "relationships_and_personal_reflection"
	TopicGamesRolePlay            TopicFine = "games_and_role_play"
	TopicAskingAboutModel         TopicFine = "asking_about_the_model"
	TopicOther                    TopicFine = "other"
	TopicUnclear                  TopicFine = "unclear"
)

// TopicCoarse represents the 7 coarse topic groups.
type TopicCoarse string

const (
	TopicCoarsePracticalGuidance TopicCoarse = "practical_guidance"
	TopicCoarseWriting           TopicCoarse = "writing"
	TopicCoarseSeekingInfo       TopicCoarse = "seeking_information"
	TopicCoarseTechnicalHelp     TopicCoarse = "technical_help"
	TopicCoarseMultimedia        TopicCoarse = "multimedia"
	TopicCoarseSelfExpression    TopicCoarse = "self_expression"
	TopicCoarseOther             TopicCoarse = "other"
)

// FineToCoarse maps fine-grained topics to their coarse groups.
var FineToCoarse = map[TopicFine]TopicCoarse{
	TopicHowToAdvice:             TopicCoarsePracticalGuidance,
	TopicTutoringOrTeaching:      TopicCoarsePracticalGuidance,
	TopicCreativeIdeation:        TopicCoarsePracticalGuidance,
	TopicHealthFitnessBeauty:     TopicCoarsePracticalGuidance,
	TopicEditOrCritique:          TopicCoarseWriting,
	TopicPersonalWriting:         TopicCoarseWriting,
	TopicTranslation:             TopicCoarseWriting,
	TopicArgumentOrSummary:       TopicCoarseWriting,
	TopicWriteFiction:            TopicCoarseWriting,
	TopicSpecificInfo:            TopicCoarseSeekingInfo,
	TopicPurchasableProducts:     TopicCoarseSeekingInfo,
	TopicCookingRecipes:          TopicCoarseSeekingInfo,
	TopicComputerProgramming:     TopicCoarseTechnicalHelp,
	TopicMathematicalCalculation: TopicCoarseTechnicalHelp,
	TopicDataAnalysis:            TopicCoarseTechnicalHelp,
	TopicCreateImage:             TopicCoarseMultimedia,
	TopicAnalyzeImage:            TopicCoarseMultimedia,
	TopicOtherMedia:              TopicCoarseMultimedia,
	TopicGreetingsChitchat:       TopicCoarseSelfExpression,
	TopicRelationshipsReflection: TopicCoarseSelfExpression,
	TopicGamesRolePlay:           TopicCoarseSelfExpression,
	TopicAskingAboutModel:        TopicCoarseOther,
	TopicOther:                   TopicCoarseOther,
	TopicUnclear:                 TopicCoarseOther,
}

// Classification holds the classification results for a single user message.
type Classification struct {
	MessageID       string      `json:"message_id"`
	ChatID          string      `json:"chat_id"`
	UserEmail       string      `json:"user_email"`
	MessageCreated  string      `json:"message_created"`
	WorkRelated     *bool       `json:"work_related,omitempty"`
	Intent          Intent      `json:"intent,omitempty"`
	TopicFine       TopicFine   `json:"topic_fine,omitempty"`
	TopicCoarse     TopicCoarse `json:"topic_coarse,omitempty"`
	ClassifiedAt    string      `json:"classified_at"`
	ClassifierModel string      `json:"classifier_model"`
}

// Classifier uses Claude to classify chat messages according to the usage taxonomy.
type Classifier struct {
	apiKey     string
	model      string
	cmd        string // Shell command to pipe prompts through (alternative to API)
	httpClient *http.Client
}

// NewClassifier creates a classifier using the given API key and model.
// Recommended models: "claude-3-5-haiku-20241022" (fast/cheap) or "claude-sonnet-4-20250514" (accurate).
func NewClassifier(apiKey, model string) *Classifier {
	if model == "" {
		model = "claude-3-5-haiku-20241022"
	}
	return &Classifier{
		apiKey:     apiKey,
		model:      model,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// NewClassifierWithCmd creates a classifier that pipes prompts through a shell command
// instead of calling the API directly. The command receives the prompt on stdin and
// should output the response on stdout.
//
// Example: NewClassifierWithCmd("claude --print", "claude-cli")
func NewClassifierWithCmd(cmd, modelName string) *Classifier {
	if modelName == "" {
		modelName = "claude-cli"
	}
	return &Classifier{
		cmd:   cmd,
		model: modelName,
	}
}

// Model returns the model name being used for classification.
func (c *Classifier) Model() string {
	return c.model
}

// UsingCmd returns true if the classifier uses a shell command instead of API.
func (c *Classifier) UsingCmd() bool {
	return c.cmd != ""
}

// ClassifyMessage classifies a single user message within its conversation context.
// The context includes preceding messages to help the classifier understand the full situation.
func (c *Classifier) ClassifyMessage(ctx context.Context, chat *ChatDetail, messageIdx int, taxonomies []string) (*Classification, error) {
	if messageIdx < 0 || messageIdx >= len(chat.ChatMessages) {
		return nil, fmt.Errorf("message index %d out of range", messageIdx)
	}

	msg := chat.ChatMessages[messageIdx]
	if msg.Role != "user" {
		return nil, fmt.Errorf("can only classify user messages, got role %q", msg.Role)
	}

	// Prefer the Rev H message ID, fall back to legacy UUID, then synthetic.
	messageID := msg.ID
	if messageID == "" {
		messageID = msg.UUID
	}
	if messageID == "" {
		messageID = fmt.Sprintf("%s_msg_%d", chat.ID, messageIdx)
	}

	result := &Classification{
		MessageID:       messageID,
		ChatID:          chat.ID,
		UserEmail:       chat.User.EmailAddress,
		MessageCreated:  msg.CreatedAt,
		ClassifiedAt:    time.Now().UTC().Format(time.RFC3339),
		ClassifierModel: c.model,
	}

	// Build context from the conversation up to and including this message.
	conversationContext := buildConversationContext(chat.ChatMessages[:messageIdx+1])

	for _, taxonomy := range taxonomies {
		switch taxonomy {
		case "work":
			isWork, err := c.classifyWorkRelated(ctx, conversationContext)
			if err != nil {
				return result, fmt.Errorf("classifying work/non-work: %w", err)
			}
			result.WorkRelated = &isWork

		case "intent":
			intent, err := c.classifyIntent(ctx, conversationContext)
			if err != nil {
				return result, fmt.Errorf("classifying intent: %w", err)
			}
			result.Intent = intent

		case "topic":
			topic, err := c.classifyTopic(ctx, conversationContext)
			if err != nil {
				return result, fmt.Errorf("classifying topic: %w", err)
			}
			result.TopicFine = topic
			result.TopicCoarse = FineToCoarse[topic]
		}
	}

	return result, nil
}

// buildConversationContext formats the conversation history for the classifier prompt.
func buildConversationContext(messages []ChatMessage) string {
	var sb strings.Builder
	for i, msg := range messages {
		role := "User"
		if msg.Role == "assistant" {
			role = "Assistant"
		}
		fmt.Fprintf(&sb, "[%s]: ", role)

		for _, c := range msg.Content {
			if c.Type == "text" {
				// Truncate very long messages to avoid context overflow.
				text := c.Text
				if len(text) > 2000 && i < len(messages)-1 {
					text = text[:2000] + "... [truncated]"
				}
				sb.WriteString(text)
			}
		}
		sb.WriteString("\n\n")
	}
	return sb.String()
}

// classifyWorkRelated determines if the message is work-related.
func (c *Classifier) classifyWorkRelated(ctx context.Context, conversationContext string) (bool, error) {
	prompt := fmt.Sprintf(`You are an internal tool that classifies a message from a user to an AI chatbot.

Does the last user message of this conversation transcript seem likely to be related to doing some work/employment? Answer with one of the following:
(1) likely part of work (e.g. "rewrite this HR complaint", "draft an email to my team", "help me with this code review")
(0) likely not part of work (e.g. "does ice reduce pimples?", "write me a poem about cats", "what should I cook for dinner?")

In your response, only give the number (1 or 0) and no other text.

Conversation transcript:
%s`, conversationContext)

	response, err := c.complete(ctx, prompt)
	if err != nil {
		return false, err
	}

	response = strings.TrimSpace(response)
	return response == "1", nil
}

// classifyIntent determines the user's intent (asking, doing, or expressing).
func (c *Classifier) classifyIntent(ctx context.Context, conversationContext string) (Intent, error) {
	prompt := fmt.Sprintf(`Assign the last user message to one of the following three categories:

- Asking: Seeking information or advice that will help the user be better informed or make better decisions. (e.g. "Who was president after Lincoln?", "How do I create a budget for this quarter?", "What's the best way to handle this situation?")

- Doing: Requests that the chatbot perform tasks for the user. User wants drafting an email, writing code, creating content, etc. (e.g. "Rewrite this email to make it more formal", "Write a Dockerfile and a minimal docker-compose.yml for this app.", "Generate a summary of this document")

- Expressing: Statements that are neither asking for information, nor for the chatbot to perform a task. Includes venting, chitchat, sharing thoughts, greetings. (e.g. "I'm so frustrated with this project", "Hello!", "That's a really interesting point")

In your response, only give the category name in lowercase (asking, doing, or expressing) and no other text.

Conversation transcript:
%s`, conversationContext)

	response, err := c.complete(ctx, prompt)
	if err != nil {
		return IntentUnknown, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	switch response {
	case "asking":
		return IntentAsking, nil
	case "doing":
		return IntentDoing, nil
	case "expressing":
		return IntentExpressing, nil
	default:
		return IntentUnknown, nil
	}
}

// classifyTopic determines the conversation topic (24 fine-grained categories).
func (c *Classifier) classifyTopic(ctx context.Context, conversationContext string) (TopicFine, error) {
	prompt := fmt.Sprintf(`Classify the last user message into one of the following 24 topic categories. Choose the single best match.

**Practical Guidance:**
- how_to_advice: Seeking advice on how to do something (e.g., "How do I negotiate a raise?")
- tutoring_or_teaching: Learning concepts, educational content (e.g., "Explain quantum computing")
- creative_ideation: Brainstorming, generating ideas (e.g., "Give me ideas for a birthday party theme")
- health_fitness_beauty_or_self_care: Health, fitness, beauty, wellness topics

**Writing:**
- edit_or_critique_provided_text: Editing, proofreading, feedback on user's text
- personal_writing_or_communication: Drafting emails, messages, personal communications
- translation: Translating between languages
- argument_or_summary_generation: Creating arguments, summaries, analysis
- write_fiction: Creative writing, stories, poems

**Seeking Information:**
- specific_info: Looking up facts, definitions, specific information
- purchasable_products: Product recommendations, shopping advice
- cooking_and_recipes: Recipes, cooking instructions, food-related

**Technical Help:**
- computer_programming: Writing, debugging, or explaining code
- mathematical_calculation: Math problems, calculations
- data_analysis: Analyzing data, statistics, creating charts

**Multimedia:**
- create_an_image: Requesting image generation
- analyze_an_image: Asking about an uploaded/shared image
- generate_or_retrieve_other_media: Audio, video, or other media requests

**Self-Expression:**
- greetings_and_chitchat: Hellos, small talk, casual conversation
- relationships_and_personal_reflection: Personal matters, relationships, emotional support
- games_and_role_play: Games, roleplay scenarios, entertainment

**Other:**
- asking_about_the_model: Questions about the AI itself, its capabilities
- other: Doesn't fit any category above
- unclear: Cannot determine from the message

In your response, only give the category identifier (e.g., "computer_programming") and no other text.

Conversation transcript:
%s`, conversationContext)

	response, err := c.complete(ctx, prompt)
	if err != nil {
		return TopicUnclear, err
	}

	response = strings.TrimSpace(strings.ToLower(response))
	// Validate the response is a known topic.
	validTopics := map[string]TopicFine{
		"how_to_advice":                     TopicHowToAdvice,
		"tutoring_or_teaching":              TopicTutoringOrTeaching,
		"creative_ideation":                 TopicCreativeIdeation,
		"health_fitness_beauty_or_self_care": TopicHealthFitnessBeauty,
		"edit_or_critique_provided_text":    TopicEditOrCritique,
		"personal_writing_or_communication": TopicPersonalWriting,
		"translation":                       TopicTranslation,
		"argument_or_summary_generation":    TopicArgumentOrSummary,
		"write_fiction":                     TopicWriteFiction,
		"specific_info":                     TopicSpecificInfo,
		"purchasable_products":              TopicPurchasableProducts,
		"cooking_and_recipes":               TopicCookingRecipes,
		"computer_programming":              TopicComputerProgramming,
		"mathematical_calculation":          TopicMathematicalCalculation,
		"data_analysis":                     TopicDataAnalysis,
		"create_an_image":                   TopicCreateImage,
		"analyze_an_image":                  TopicAnalyzeImage,
		"generate_or_retrieve_other_media":  TopicOtherMedia,
		"greetings_and_chitchat":            TopicGreetingsChitchat,
		"relationships_and_personal_reflection": TopicRelationshipsReflection,
		"games_and_role_play":               TopicGamesRolePlay,
		"asking_about_the_model":            TopicAskingAboutModel,
		"other":                             TopicOther,
		"unclear":                           TopicUnclear,
	}

	if topic, ok := validTopics[response]; ok {
		return topic, nil
	}
	return TopicUnclear, nil
}

// complete sends a prompt and returns the response, using either a shell command or API.
func (c *Classifier) complete(ctx context.Context, prompt string) (string, error) {
	if c.cmd != "" {
		return c.completeViaCmd(ctx, prompt)
	}
	return c.completeViaAPI(ctx, prompt)
}

// completeViaCmd pipes the prompt through a shell command.
func (c *Classifier) completeViaCmd(ctx context.Context, prompt string) (string, error) {
	cmd := exec.CommandContext(ctx, "sh", "-c", c.cmd)
	cmd.Stdin = strings.NewReader(prompt)

	output, err := cmd.Output()
	if err != nil {
		if exitErr, ok := err.(*exec.ExitError); ok {
			return "", fmt.Errorf("command failed: %s\nstderr: %s", err, string(exitErr.Stderr))
		}
		return "", fmt.Errorf("command failed: %w", err)
	}

	return strings.TrimSpace(string(output)), nil
}

// completeViaAPI calls the Claude API directly.
func (c *Classifier) completeViaAPI(ctx context.Context, prompt string) (string, error) {
	reqBody := map[string]interface{}{
		"model":      c.model,
		"max_tokens": 50,
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
	}

	jsonBody, err := json.Marshal(reqBody)
	if err != nil {
		return "", fmt.Errorf("marshaling request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, "https://api.anthropic.com/v1/messages", bytes.NewReader(jsonBody))
	if err != nil {
		return "", err
	}
	req.Header.Set("x-api-key", c.apiKey)
	req.Header.Set("anthropic-version", "2023-06-01")
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("API request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("reading response: %w", err)
	}

	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("API error %d: %s", resp.StatusCode, truncate(string(body), 200))
	}

	var result struct {
		Content []struct {
			Type string `json:"type"`
			Text string `json:"text"`
		} `json:"content"`
	}
	if err := json.Unmarshal(body, &result); err != nil {
		return "", fmt.Errorf("parsing response: %w", err)
	}

	if len(result.Content) == 0 {
		return "", fmt.Errorf("empty response from API")
	}

	return result.Content[0].Text, nil
}

// ClassifyChat classifies all user messages in a chat.
func (c *Classifier) ClassifyChat(ctx context.Context, chat *ChatDetail, taxonomies []string) ([]*Classification, error) {
	var results []*Classification

	for i, msg := range chat.ChatMessages {
		if msg.Role != "user" {
			continue
		}

		classification, err := c.ClassifyMessage(ctx, chat, i, taxonomies)
		if err != nil {
			return results, fmt.Errorf("classifying message %d: %w", i, err)
		}
		results = append(results, classification)
	}

	return results, nil
}

// AllTaxonomies returns the list of all available taxonomy dimensions.
func AllTaxonomies() []string {
	return []string{"work", "intent", "topic"}
}
