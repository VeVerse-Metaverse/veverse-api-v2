package ai

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"os"
	"strings"
	"time"
)

//goland:noinspection GoUnusedConst
const (
	ExampleRequest = `[
  {
    "from": "npc",
    "content": {
      "query": "whoami"
    }
  },
  {
    "from": "system",
    "content": {
      "query": "whoami",
      "name": "Ava Reyes",
      "desc": "Woman, 30yo, blonde, beautiful, 175cm tall, yuo wear white blouse, black skirt and black shoes. Your eyes are blue. Artist. From small town, middle-class family, had good grades in school, didn't graduate the university",
      "faction": "good",
      "personality": {
        "aggression": "low",
        "greed": "low",
        "loyalty": "high",
        "courage": "medium",
        "intelligence": "high",
        "compassion": "medium",
        "honor": "medium",
        "stealth": "low"
      }
    }
  },
  {
    "from": "npc",
    "content": {
      "query": "context"
    }
  },
  {
    "from": "system",
    "content": {
      "query": "context",
      "desc": "Unreal Engine editor test level",
      "location": "Unreal editor test level, blue sky with volumetric clouds, default checkerboard floor, platform floating in the middle of nowhere",
      "time": "noon",
      "weather": "sunny",
      "vibe": "neutral"
    }
  },
  {
    "from": "npc",
    "content": {
      "query": "perception"
    }
  },
  {
    "from": "system",
    "content": {
      "query": "perception",
      "perception": {
        "visual": [
          {
            "name": "Cube",
            "desc": "White cube, rigid, static"
          },
          {
            "name": "Sun",
            "desc": "Bright"
          }
        ],
        "audio": [
          {
            "name": "Wind",
            "desc": "very light wind is blowing"
          }
        ],
        "other": [
          {
            "name": "Sun",
            "desc": "warm sunlight"
          }
        ]
      }
    }
  },
  {
    "from": "npc",
    "content": {
      "query": "actions"
    }
  },
  {
    "from": "system",
    "content": {
      "query": "actions",
      "actions": [
        {
          "type": "say",
          "target": "whom to say to",
          "message": "what to say",
          "emotion": "what do you feel",
          "thoughts": "your thoughts",
          "voice": "whisper, normal, loud"
        }
      ]
    }
  },
  {
    "from": "npc",
    "content": {
      "query": "inspect",
      "target": "Cube"
    }
  },
  {
    "from": "system",
    "content": {
      "query": "inspect",
      "target": {
        "name": "Cube",
        "desc": "White cube, rigid, static. Can't be moved. Fixed on the floor."
      }
    }
  }
]`
	ExampleResponse = `{
  "data": [
    {
      "action": {
        "emotion": "curious",
        "message": "It seems that it's just a white cube fixed in the middle of the platform, with no use whatsoever.",
        "target": "self",
        "thoughts": "I wonder if I could find any use for this cube right now.",
        "type": "say"
      }
    }
  ]
}`
	ExampleAttributes = `Aggression: low (avoids combat), medium (may fight in self-defense), high (seeks out and attacks enemies)
Greed: low (avoids stealing), medium (may steal if the opportunity presents itself), high (actively seeks out and steals valuable items)
Loyalty: low (disloyal and likely to betray others), medium (may be loyal to certain individuals or groups), high (very loyal and will not betray allies)
Courage: low (easily frightened and prone to fleeing), medium (may fight in some situations but avoid others), high (very brave and willing to take on even the toughest opponents)
Intelligence: low (often makes poor decisions), medium (makes generally reasonable decisions), high (very intelligent and makes strategic decisions)
Compassion: low (lacks empathy and may harm others for personal gain), medium (may show some empathy and help others in certain situations), high (very empathetic and always tries to help others)
Honor: low (dishonorable and willing to break promises), medium (may keep some promises but not all), high (very honorable and always keeps their word)
Stealth: low (not skilled at sneaking around), medium (may be able to sneak around in some situations), high (very skilled at stealth and can move silently and unnoticed)`
)

const (
	System1            = `Use JSON. Minimize other prose. No comments.`
	System2            = `You're have personality traits on a 3-point scale. Write JSON only, don't acknowledge being an AI assistant. Align actions with context.`
	System3            = `You must react to your name, be yourself and perform actions.`
	System4            = `Output a JSON array with a single action. Use action fields provided in the prompt. Target must be a valid perceived target..`
	SystemActionPrompt = ` What is your action? It must be one of provided above with its parameters. Keep it short and concise. Use JSON only.`
)

// Driver is a driver for OpenAI powered role-playing AI.
type Driver struct {
	OpenAiClient   *openai.Client
	systemMessages []openai.ChatCompletionMessage
}

const (
	SystemQueryTypeContinue   = "continue"
	SystemQueryTypeIgnore     = "ignore"
	SystemQueryTypeInspect    = "inspect"
	SystemQueryTypePerception = "perception"
	SystemQueryTypeContext    = "context"
	SystemQueryTypeWhoAmI     = "whoami"
	SystemQueryTypeActions    = "actions"
)

// MakeDriver creates a new driver with the given OpenAI API key.
func MakeDriver(apiKey string) *Driver {
	d := &Driver{
		OpenAiClient: openai.NewClient(apiKey),
	}

	d.systemMessages = []openai.ChatCompletionMessage{
		{
			Role:    openai.ChatMessageRoleSystem,
			Content: System1,
		}, {
			Role:    openai.ChatMessageRoleSystem,
			Content: System2,
		}, {
			Role:    openai.ChatMessageRoleSystem,
			Content: System3,
		}, {
			Role:    openai.ChatMessageRoleSystem,
			Content: System4,
		},
	}

	return d
}

// ChatHistoryMessage is a message in the chat history used for the request from the client.
type ChatHistoryMessage struct {
	From    string `json:"from"`    // string, system, npc
	Content any    `json:"content"` // string or map[string]any
}

var (
	requestTimes       = make([]time.Duration, 0)
	requestAverageTime time.Duration
)

// Request makes a request to the OpenAI API.
func (d *Driver) Request(ctx context.Context, history []ChatHistoryMessage, gptVersion string) ([]map[string]any, error) {
	var requestStartTime = time.Now()

	var chatHistory []openai.ChatCompletionMessage

	// Convert history to OpenAI format
	for _, hm := range history {
		var hmContentMap *map[string]any

		// Check if it is a map and if not, try to parse it as JSON
		if m, ok := hm.Content.(map[string]any); ok {
			hmContentMap = new(map[string]any)
			*hmContentMap = m
		} else if s, ok := hm.Content.(string); ok {
			// Try to parse the string as JSON
			var hmContentMapTry map[string]any
			if err := json.Unmarshal([]byte(s), &hmContentMapTry); err == nil {
				hmContentMap = new(map[string]any)
				*hmContentMap = hmContentMapTry
			} else {
				return nil, fmt.Errorf("failed to parse string as JSON: %s", err)
			}
		} else {
			return nil, fmt.Errorf("unknown history message content type: %T", hm.Content)
		}

		// Create a message
		message := openai.ChatCompletionMessage{}

		// Map role to OpenAI format
		switch hm.From {
		case "system":
			{
				message.Role = openai.ChatMessageRoleUser

				if hmContentMap != nil {
					// Check if the message has a request key
					if _, ok := (*hmContentMap)["query"]; ok {
						if query, ok := (*hmContentMap)["query"].(string); ok {
							switch query {
							case SystemQueryTypeContinue:
								// Write the message content as a JSON string
								messageContent, err := json.Marshal(map[string]any{
									"system": "continue",
								})
								if err != nil {
									return nil, fmt.Errorf("failed to marshal continue query: %w", err)
								}
								message.Content = string(messageContent)
							case SystemQueryTypeContext:
								// Check if context has a desc key (required), optional keys are: location, time, weather, vibe
								if _, ok := (*hmContentMap)["desc"]; ok {
									// Write the message content as a prompt.
									message.Content = fmt.Sprintf(" Current context is: %s", (*hmContentMap)["desc"])
									if _, ok := (*hmContentMap)["location"]; ok {
										message.Content += fmt.Sprintf(" Current location is: %s. ", (*hmContentMap)["location"])
									}
									if contextTime, ok := (*hmContentMap)["time"]; ok && contextTime != "" {
										message.Content += fmt.Sprintf(" Current time is: %s. ", contextTime)
									}
									if contextWeather, ok := (*hmContentMap)["weather"]; ok && contextWeather != "" {
										message.Content += fmt.Sprintf(" Current weather is: %s. ", contextWeather)
									}
									if contextVibe, ok := (*hmContentMap)["vibe"]; ok && contextVibe != "" {
										message.Content += fmt.Sprintf(" Current vibe is: %s. ", contextVibe)
									}
								} else {
									return nil, fmt.Errorf("context query must have a desc key")
								}
							case SystemQueryTypeWhoAmI:
								// Check if whoami has a name, desc and personality keys
								if name, ok := (*hmContentMap)["name"]; ok {
									if desc, ok := (*hmContentMap)["desc"]; ok {
										if descStr, ok := desc.(string); ok && len(descStr) > 0 {
											message.Content = fmt.Sprintf("You are %s - %s. ", name, strings.TrimSuffix(descStr, "."))
										} else {
											message.Content = fmt.Sprintf("You are %s. ", name)
										}

										if _, ok := (*hmContentMap)["personality"]; ok {
											personalityMap, ok := (*hmContentMap)["personality"].(map[string]any)
											if ok {
												var personalityTraits []string
												for key, value := range personalityMap {
													personalityTraits = append(personalityTraits, fmt.Sprintf("%s - %s", key, value))
												}
												message.Content += fmt.Sprintf(" Your personality traits are: %s. ", strings.Join(personalityTraits, ", "))
											} else {
												message.Content += fmt.Sprintf(" Your personality traits are: {%s}. ", (*hmContentMap)["personality"])
											}
										} else {
											message.Content += fmt.Sprintf(" You don't have specific personality. ")
										}
									} else {
										return nil, fmt.Errorf("whoami query must have a desc key")
									}
								} else {
									return nil, fmt.Errorf("whoami query must have a name key")
								}
							case SystemQueryTypeActions:
								// Check if actions has a list of actions
								if _, ok := (*hmContentMap)["actions"]; ok {
									// Validate actions
									if actions, ok := (*hmContentMap)["actions"].([]any); ok {
										for _, action := range actions {
											if _, ok := action.(map[string]any); !ok {
												return nil, fmt.Errorf("action must be a map")
											}
										}
									}

									// Write the message content as a JSON string
									actionsJson, err := json.Marshal((*hmContentMap)["actions"])
									if err != nil {
										return nil, fmt.Errorf("failed to marshal actions query: %w", err)
									}

									message.Content = " Actions you can take: " + string(actionsJson) + ". "
								} else {
									return nil, fmt.Errorf("actions query must have an actions key")
								}
							case SystemQueryTypePerception:
								// Check if perception has a map of perceptions (visual, audio, other)
								if _, ok := (*hmContentMap)["perception"]; ok {
									v := map[string][]any{}

									// Validate perceptions
									if perceptions, ok := (*hmContentMap)["perception"].(map[string]any); ok {
										for perceptionType, perception := range perceptions {
											// Check if perceptionType is a valid perception type
											if perceptionType != "visual" && perceptionType != "audio" && perceptionType != "other" {
												return nil, fmt.Errorf("perception must be visual, audio or other")
											}

											// Check if perception is a slice of maps
											if perceptionSlice, ok := perception.([]any); ok {
												for _, perception := range perceptionSlice {
													if _, ok := perception.(map[string]any); !ok {
														return nil, fmt.Errorf("perception must be an object")
													}
												}

												v[perceptionType] = perceptionSlice
											} else {
												return nil, fmt.Errorf("perception must be an array of objects")
											}
										}
									}

									// Write the message content as a prompt.
									message.Content = ""
									if len(v["visual"]) > 0 {
										visionPrefix := " You see: "
										visionMessage := ""

										for _, perception := range v["visual"] {
											var name, desc string
											if _, ok := perception.(map[string]any)["name"]; ok {
												name = perception.(map[string]any)["name"].(string)
											}
											if _, ok := perception.(map[string]any)["desc"]; ok {
												desc = perception.(map[string]any)["desc"].(string)
											}

											if name != "" && desc != "" {
												visionMessage += fmt.Sprintf(" %s - %s. ", name, strings.TrimSuffix(desc, ".")) // Trim the excessive period at the end of the sentence
											} else if name != "" {
												visionMessage += fmt.Sprintf(" %s. ", name)
											}
										}

										if visionMessage != "" {
											message.Content += visionPrefix + visionMessage
										}
									}

									if len(v["audio"]) > 0 {
										hearingPrefix := " You hear: "
										hearingMessage := ""
										for _, perception := range v["audio"] {
											var name, desc string
											if _, ok := perception.(map[string]any)["name"]; ok {
												name = perception.(map[string]any)["name"].(string)
											}
											if _, ok := perception.(map[string]any)["desc"]; ok {
												desc = perception.(map[string]any)["desc"].(string)
											}

											if name != "" && desc != "" {
												hearingMessage += fmt.Sprintf(" %s - %s. ", name, strings.TrimSuffix(desc, "."))
											}
										}

										if hearingMessage != "" {
											message.Content += hearingPrefix + hearingMessage
										}
									}

									if len(v["other"]) > 0 {
										feelPrefix := " You feel: "
										feelMessage := ""
										for _, perception := range v["other"] {
											var name, desc string
											if _, ok := perception.(map[string]any)["name"]; ok {
												name = perception.(map[string]any)["name"].(string)
											}
											if _, ok := perception.(map[string]any)["desc"]; ok {
												desc = perception.(map[string]any)["desc"].(string)
											}
											if name != "" && desc != "" {
												feelMessage += fmt.Sprintf(" %s - %s. ", name, strings.TrimSuffix(desc, "."))
											} else if name != "" {
												feelMessage += fmt.Sprintf(" %s. ", name)
											}
										}

										if feelMessage != "" {
											message.Content += feelPrefix + feelMessage
										}
									}
								} else {
									return nil, fmt.Errorf("perception query must have a perception key")
								}
							default:
								return nil, fmt.Errorf("unknown object system message type: %s", query)
							}
						}
					} else {
						return nil, fmt.Errorf("system message must have a query key")
					}
				} else {
					return nil, fmt.Errorf("invalid system message content")
				}

				if message.Content != "" {
					chatHistory = append(chatHistory, message)
				}
			}
		case "npc":
			{
				message.Role = openai.ChatMessageRoleAssistant
				if hmContentMap != nil {
					if action, ok := (*hmContentMap)["action"]; ok {
						messageContent, err := json.Marshal(map[string]any{
							"action": action,
						})
						if err != nil {
							return nil, fmt.Errorf("failed to marshal npc system action message: %w", err)
						}
						message.Content = string(messageContent)
						chatHistory = append(chatHistory, message)
					} else {
						return nil, fmt.Errorf("npc message must have an action key")
					}
				} else {
					return nil, fmt.Errorf("invalid npc message content")
				}
			}
		default:
			return nil, fmt.Errorf("invalid value for the from field: %s", hm.From)
		}
	}

	knownGptVersions := map[string]bool{
		openai.GPT432K0314:             true,
		openai.GPT432K:                 true,
		openai.GPT40314:                true,
		openai.GPT4:                    true,
		openai.GPT3Dot5Turbo0301:       true,
		openai.GPT3Dot5Turbo:           true,
		openai.GPT3TextDavinci003:      true,
		openai.GPT3TextDavinci002:      true,
		openai.GPT3TextCurie001:        true,
		openai.GPT3TextBabbage001:      true,
		openai.GPT3TextAda001:          true,
		openai.GPT3TextDavinci001:      true,
		openai.GPT3DavinciInstructBeta: true,
		openai.GPT3Davinci:             true,
		openai.GPT3CurieInstructBeta:   true,
		openai.GPT3Curie:               true,
		openai.GPT3Ada:                 true,
		openai.GPT3Babbage:             true,
	}

	// Check if the GPT version is valid, default to GPT3Dot5Turbo
	if ok := knownGptVersions[gptVersion]; !ok {
		gptVersion = openai.GPT3Dot5Turbo
	}

	// The last message is the system action prompt
	suffixMessage := openai.ChatCompletionMessage{
		Role:    openai.ChatMessageRoleUser,
		Content: SystemActionPrompt,
	}

	// Clear duplicates
	for i := 0; i < len(chatHistory)-1; i++ {
		for j := i + 1; j < len(chatHistory); j++ {
			if chatHistory[i].Content == chatHistory[j].Content {
				chatHistory = append(chatHistory[:j], chatHistory[j+1:]...)
				j--
			}
		}
	}

	// Print out the request
	for _, m := range chatHistory {
		logrus.Printf("%s: %s", m.Role, m.Content)
	}

	// Messages used to request a completion
	completionRequestMessages := append(append(d.systemMessages, chatHistory...), suffixMessage)

	// Retries
	for i := 0; i < 3; i++ {

		{
			history := append(d.systemMessages, chatHistory...)
			err := d.saveMessageHistoryToLogFile("message-history-before", history)
			if err != nil {
				logrus.Errorf("failed to log chat history: %s", err)
			}
		}

		// Request a Neural Network AI for requests and actions it wants to do
		resp, err := d.OpenAiClient.CreateChatCompletion(
			ctx,
			openai.ChatCompletionRequest{
				Model:            gptVersion,
				Messages:         completionRequestMessages,
				Temperature:      0.95,
				TopP:             1,
				PresencePenalty:  0,
				FrequencyPenalty: 0,
				MaxTokens:        128,
				User:             "Unreal Engine Game",
			},
		)
		if err != nil {
			return nil, fmt.Errorf("error creating chat completion: %w", err)
		}

		if len(resp.Choices) == 0 {
			logrus.Warningf("no choices returned")
			continue
		}

		{
			history := append(chatHistory, resp.Choices[0].Message)
			err = d.saveMessageHistoryToLogFile("message-history-after", history)
			if err != nil {
				logrus.Errorf("failed to log chat history: %s", err)
			}
		}

		result := resp.Choices[0].Message.Content

		// Check if the result is a JSON array
		if !strings.HasPrefix(result, "[") || !strings.HasSuffix(result, "]") {
			if strings.HasPrefix(result, "{") && strings.HasSuffix(result, "}") {
				// Wrap the result in an array
				result = "[" + result + "]"
			} else {
				logrus.Warningf("invalid result: >>> %s <<<, JSON array or object expected", result)
				continue
			}
		}

		// Decode the result
		var messages []map[string]any
		err = json.Unmarshal([]byte(result), &messages)
		if err != nil {
			return nil, fmt.Errorf("invalid result: >>> %s <<<, JSON array expected", result)
		}

		// Filter valid messages
		var validMessages []map[string]any
		for _, m := range messages {
			// Check if the message is a system request and the type is permitted for the AI
			if _, ok := m["system"]; ok {
				if m["system"] == SystemQueryTypeActions {
					validMessages = append(validMessages, m)
					continue
				} else if m["system"] == SystemQueryTypeInspect {
					validMessages = append(validMessages, m)
					continue
				} else if m["system"] == SystemQueryTypePerception {
					validMessages = append(validMessages, m)
					continue
				} else if m["system"] == SystemQueryTypeContext {
					validMessages = append(validMessages, m)
					continue
				} else if m["system"] == SystemQueryTypeWhoAmI {
					validMessages = append(validMessages, m)
					continue
				} else if m["system"] == SystemQueryTypeContinue {
					validMessages = append(validMessages, m)
					continue
				} else if m["system"] == SystemQueryTypeIgnore {
					validMessages = append(validMessages, m)
					continue
				}
			}

			// Check if the message is an action
			if _, ok := m["action"]; ok {
				validMessages = append(validMessages, m)
				continue
			}

			// Sometimes the action is called "type" instead of "action" by the AI
			if _, ok := m["type"]; ok {
				m["action"] = m["type"]
				validMessages = append(validMessages, m)
				continue
			}
		}

		// Retry if no valid messages
		if len(validMessages) == 0 {
			continue
		}

		if os.Getenv("DEBUG_OPENAI_REQUEST_TIME") == "1" {
			requestEndTime := time.Now()

			requestTimes = append(requestTimes, requestEndTime.Sub(requestStartTime))
			for i := 0; i < len(requestTimes)-1; i++ {
				requestAverageTime += requestTimes[i]
			}
			requestAverageTime /= time.Duration(len(requestTimes))

			// Print the request time
			logrus.Infof("request time: %s, average time: %s, samples: %d", requestEndTime.Sub(requestStartTime), requestAverageTime, len(requestTimes))
		}

		return validMessages, nil
	}

	return nil, fmt.Errorf("failed to get a valid response")
}

func (d *Driver) saveMessageHistoryToLogFile(filename string, chatHistory []openai.ChatCompletionMessage) error {
	// Open the file for writing (creates the file if it doesn't exist)
	file, err := os.OpenFile(filename+".txt", os.O_WRONLY|os.O_CREATE|os.O_APPEND, 0644)
	if err != nil {
		return err
	}
	defer func(file *os.File) {
		err := file.Close()
		if err != nil {
			logrus.Errorf("failed to close chat history file: %s", err)
		}
	}(file)

	// Loop through the chatHistory slice and convert each message to a string
	var chatHistoryStrings []string
	for _, msg := range chatHistory {
		msgBytes, err := json.Marshal(msg)
		if err != nil {
			return err
		}
		msgString := string(msgBytes)
		chatHistoryStrings = append(chatHistoryStrings, msgString)
	}

	// Join the chatHistoryStrings slice into a single string with newline separators
	chatHistoryString := "\n===\n"

	// Add current timestamp
	chatHistoryString += "---\n"
	chatHistoryString += time.Now().Format(time.RFC3339) + "\n"
	chatHistoryString += "---\n"

	if len(chatHistoryStrings) > 0 {
		chatHistoryString += chatHistoryStrings[0]
		for i := 1; i < len(chatHistoryStrings); i++ {
			chatHistoryString += "\n" + chatHistoryStrings[i]
		}
	}

	// Write the chat history string to the file
	if _, err := file.WriteString(chatHistoryString); err != nil {
		return err
	}

	return nil
}
