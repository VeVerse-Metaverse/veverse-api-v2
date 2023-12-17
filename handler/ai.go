package handler

import (
	"bytes"
	speech "cloud.google.com/go/speech/apiv1"
	speechpb "cloud.google.com/go/speech/apiv1/speechpb"
	texttospeech "cloud.google.com/go/texttospeech/apiv1"
	vContext "dev.hackerman.me/artheon/veverse-shared/context"
	sm "dev.hackerman.me/artheon/veverse-shared/model"
	"encoding/json"
	"fmt"
	"github.com/gofiber/fiber/v2"
	"github.com/gofrs/uuid"
	"github.com/sashabaranov/go-openai"
	"github.com/sirupsen/logrus"
	"github.com/tailscale/hujson"
	"io"
	"log"
	"mime/multipart"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"text/template"
	"veverse-api/ai"
	"veverse-api/aws/s3"
	"veverse-api/helper"
)

const GenericRequest = `
I give you a JSON describing a virtual world composed of entities: Locations, NPCs, Players and Objects.
You give me a play act script in return so we can play it in the video game or movie.
NPCs, Players and Objects are in locations.
NPCs can speak to Players and each other.
Play act is a FSM states making NPCs perform actions one by one in order.
State actions are simple such as "say" used to say a simple phrase or even a short story from their life, "use", etc.
Most actions have a target, e.g. say action target is a subject to say to.
The say action metadata should be in text format.
Action object is the object used to perform the action on target.
Metadata is additional data on the action such as phrase to say or other important data.
Each action must have an NPC who performs it.
NPCs can't use objects they don't have, but they can do a spawn action to pop in new objects.
I provide an input with description of the scene and entities.
Output must be list of actions, performed only by NPCs.
Give me output with the play script.
Minimise other prose except the script JSON.
Output JSON must have no trailing commas.
Empty fields (nulls or empty strings) must be omitted.
Each state output fields supported are:
- a - action type, string;
- c - NPC performing the action, string;
- t - target of the action, string;
- o - item used in the action, string;
- m - additional data on the action such as a phrase to say, string;
- e - a text smile, string;
NPCs spawn in locations as defined in input and can go to other locations.
NPCs should mostly speak to each other sometimes using jokes and sarcasm and react to these.
NPCs can emote within a say action or as a separate emote action.
---
Input:
{{.Input}}`

func GetAiSimpleFsmStates(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := sm.AiSimpleFsmStatesRequestV2{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion
	if m.States < 10 {
		m.States = 10
	}

	// Convert m to JSON string
	mJson, err := json.Marshal(m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	s := string(mJson)

	data := struct {
		Input string
	}{
		Input: s,
	}

	// Create the template
	t, err := template.New("request").Parse(GenericRequest)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Process the template
	var tpl bytes.Buffer
	err = t.Execute(&tpl, data)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Get the AI client
	driver, ok := c.UserContext().Value(vContext.AI).(*ai.Driver)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "ai features not enabled", "data": nil})
	}

	request := tpl.String()

	resp, err := driver.OpenAiClient.CreateChatCompletion(
		c.UserContext(),
		openai.ChatCompletionRequest{
			Model:       openai.GPT4,
			Temperature: 0.95,
			Messages: []openai.ChatCompletionMessage{
				{
					Role:    openai.ChatMessageRoleUser,
					Content: request,
				},
			},
		},
	)

	if err != nil {
		fmt.Printf("ChatCompletion error: %v\n", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if len(resp.Choices) == 0 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate a simple FSM states", "data": nil})
	}

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	content := strings.TrimSpace(resp.Choices[0].Message.Content)

	// Extract the JSON from the response if it has some extra text
	if !strings.HasPrefix(content, "[") || !strings.HasSuffix(content, "]") {
		// Find the first index of the [ character
		start := strings.Index(content, "[")
		if start == -1 {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate a simple FSM states", "data": nil})
		} else {
			// Find the last index of the ] character
			end := strings.LastIndex(content, "]")
			if end == -1 {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "failed to generate a simple FSM states", "data": nil})
			} else {
				content = content[start : end+1]
			}
		}
	}

	req := sm.CreateAiSimpleFsmRequest{Text: content}
	err = sm.CreateAiSimpleFsmScript(c.UserContext(), requester, req)
	if err != nil {
		return err
	}

	var b []byte
	b, err = hujson.Standardize([]byte(content))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	content = string(b)

	// Parse the content as JSON as a map
	var contentMap []map[string]interface{}
	err = json.Unmarshal([]byte(content), &contentMap)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": contentMap})
}

func AiTextToSpeech(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := sm.AiTextToSpeechRequest{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	// Validate text
	if m.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no text", "data": nil})
	}

	// Validate language code
	if m.LanguageCode == "" {
		m.LanguageCode = sm.AiTextToSpeechRequestDefaultLanguageCode
	} else if !sm.SupportedAiTextToSpeechLanguageCodes[m.LanguageCode] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "unsupported language code", "data": m.LanguageCode})
	}

	// Validate voice name
	//if m.VoiceId == "" {
	//	m.VoiceId = sm.AiTextToSpeechRequestDefaultVoiceId
	//} else if !sm.SupportedAiTextToSpeechVoiceIds[m.VoiceId] {
	//	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "unsupported voice id", "data": m.VoiceId})
	//}

	// Validate engine
	if m.Engine == "" {
		m.Engine = sm.AiTextToSpeechRequestDefaultEngine
	} else if !sm.SupportedAiTextToSpeechEngines[m.Engine] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "unsupported engine", "data": m.Engine})
	}

	// Validate sample rate
	if m.SampleRate == "" {
		m.SampleRate = sm.AiTextToSpeechRequestDefaultSampleRate
	} else if !sm.SupportedAiTextToSpeechSampleRates[m.SampleRate] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "unsupported sample rate", "data": m.SampleRate})
	}

	// Validate SpeechMarkTypes
	if len(m.SpeechMarkTypes) > 4 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "too many speech mark types", "data": nil})
	}
	if len(m.SpeechMarkTypes) > 0 {
		for _, speechMarkType := range m.SpeechMarkTypes {
			if !sm.SupportedAiTextToSpeechSpeechMarkTypes[speechMarkType] {
				return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "unsupported speech mark type", "data": speechMarkType})
			}
		}
	}

	// Validate text type
	if m.TextType == "" {
		m.TextType = sm.AiTextToSpeechRequestDefaultTextType
	} else if !sm.SupportedAiTextToSpeechTextTypes[m.TextType] {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "unsupported text type", "data": m.TextType})
	}

	if m.TextType == "ssml" {
		// Validate SSML
		if !strings.HasPrefix(m.Text, "<speak>") || !strings.HasSuffix(m.Text, "</speak>") {
			m.TextType = "text"
		}
	}

	tts, ok := c.UserContext().Value(vContext.TTS).(*texttospeech.Client)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "ai features not enabled", "data": nil})
	}

	//resp, err := sm.RequestPollyAudio(tts, m)
	resp, err := sm.RequestGoogleCloudTtsAudio(tts, m)

	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Generate a unique id for the audio
	id, err := uuid.NewV4()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	format := ""

	mimeType := "application/octet-stream"
	if m.OutputFormat == "ogg_vorbis" {
		format = ".ogg"
		mimeType = "audio/ogg"
	} else if m.OutputFormat == "pcm" {
		format = ".wav"
		mimeType = "audio/wav"
	} else if m.OutputFormat == "mp3" {
		format = ".mp3"
		mimeType = "audio/mp3"
	} else if m.OutputFormat == "json" {
		format = ".json"
		mimeType = "application/json"
	}
	// Generate the key for the audio
	key := fmt.Sprintf("tts/%s%s", id.String(), format)
	metadata := map[string]string{
		"type": "tts",
	}
	// Upload audio bytes to AWS S3
	err = s3.UploadObject(key, resp, mimeType, true, &metadata, nil)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Get presigned url for the audio
	url := s3.GetS3UrlForFile(key)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Return the audio url
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": url})
}

func AiTextToSpeechElevenLabsCached(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := sm.AiTextToSpeechRequest{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	voiceMap := map[string]string{
		"en-US-Neural2-C": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-E": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-F": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-G": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-H": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-A": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-D": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-I": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-J": "xxxxxxxxxxxxxxxxxxxx",
	}

	voice := voiceMap[m.VoiceId]

	// Validate text
	if m.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no text", "data": nil})
	}

	payloadBytes, err := json.Marshal(map[string]any{
		"text":     m.Text,
		"model_id": "eleven_monolingual_v1",
		"voice_settings": map[string]any{
			"stability":        0.85,
			"similarity_boost": 0.5,
		},
	})

	var apiKey = os.Getenv("ELEVENLABS_API_KEY")

	req, err := http.NewRequest("POST", "https://api.elevenlabs.io/v1/text-to-speech/"+voice, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if resp.StatusCode != 200 {
		// ready body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		// close body
		err = resp.Body.Close()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "error from elevenlabs", "data": body})
	}

	// Generate a unique id for the audio
	id, err := uuid.NewV4()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	format := ""

	mimeType := "application/octet-stream"
	if m.OutputFormat == "ogg_vorbis" {
		format = ".ogg"
		mimeType = "audio/ogg"
	} else if m.OutputFormat == "pcm" {
		format = ".wav"
		mimeType = "audio/wav"
	} else if m.OutputFormat == "mp3" {
		format = ".mp3"
		mimeType = "audio/mp3"
	} else if m.OutputFormat == "json" {
		format = ".json"
		mimeType = "application/json"
	}
	// Generate the key for the audio
	key := fmt.Sprintf("tts/%s%s", id.String(), format)
	metadata := map[string]string{
		"type": "tts",
	}
	// Upload audio bytes to AWS S3
	err = s3.UploadObject(key, resp.Body, mimeType, true, &metadata, nil)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Get presigned url for the audio
	url := s3.GetS3UrlForFile(key)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Return the audio url
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": url})
}

func AiTextToSpeechElevenLabs(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	//region Request metadata

	// Parse batch request metadata from the request
	m := sm.AiTextToSpeechRequest{}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//endregion

	voiceMap := map[string]string{
		"en-US-Neural2-C": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-E": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-F": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-G": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-H": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-A": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-D": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-I": "xxxxxxxxxxxxxxxxxxxx",
		"en-US-Neural2-J": "xxxxxxxxxxxxxxxxxxxx",
	}

	voice := voiceMap[m.VoiceId]

	// Validate text
	if m.Text == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "no text", "data": nil})
	}

	payloadBytes, err := json.Marshal(map[string]any{
		"text":     m.Text,
		"model_id": "eleven_monolingual_v1",
		"voice_settings": map[string]any{
			"stability":        0.85,
			"similarity_boost": 0.5,
		},
	})

	var apiKey = os.Getenv("ELEVENLABS_API_KEY")

	req, err := http.NewRequest("POST", "https://api.elevenlabs.io/v1/text-to-speech/"+voice, bytes.NewBuffer(payloadBytes))
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	req.Header.Set("xi-api-key", apiKey)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{}
	resp, err := client.Do(req)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	if resp.StatusCode != 200 {
		// ready body
		body, err := io.ReadAll(resp.Body)
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		// close body
		err = resp.Body.Close()
		if err != nil {
			return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
		}
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": "error from elevenlabs", "data": body})
	}

	// get app directory
	executablePath, err := os.Executable()
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}
	appDirectory := filepath.Dir(executablePath)
	tempDirectory := filepath.Join(appDirectory, "temp")
	err = os.Mkdir(tempDirectory, 0750)
	if err != nil && !os.IsExist(err) {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	buf, err := io.ReadAll(resp.Body)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	//tempFile, err := os.CreateTemp(tempDirectory, "elevenlabs-*.mp3")
	//if err != nil {
	//	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	//}
	//_, err = io.Copy(tempFile, resp.Body)
	//if err != nil {
	//	return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	//}
	//
	//err = tempFile.Close()
	//if err != nil {
	//	logrus.Errorf("error closing temp file: %s", err.Error())
	//}
	//
	//defer func(tempFile *os.File) {
	//	err := os.Remove(tempFile.Name())
	//	if err != nil {
	//		logrus.Errorf("error removing temp file: %s", err.Error())
	//	}
	//}(tempFile)

	return c.Status(fiber.StatusOK).Send(buf)
}

func AiSpeechToText(c *fiber.Ctx) error {

	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Warningf("%d: no requester", fiber.StatusForbidden)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		logrus.Warningf("%d: banned", fiber.StatusForbidden)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	// Process uploaded file

	//region Process multipart form uploaded file

	// Get upload file
	formFile, err := c.FormFile("file")
	if err != nil {
		return err
	}

	// Get upload file buffer
	buffer, err := formFile.Open()
	if err != nil {
		return err
	}
	defer func(buffer multipart.File) {
		err := buffer.Close()
		if err != nil {
			logrus.Errorf("error closing multipart file: %v", err)
		}
	}(buffer)
	//endregion

	ctx := c.UserContext()

	// Create a new Speech client
	client, err := speech.NewClient(ctx)
	if err != nil {
		log.Fatalf("Failed to create client: %v", err)
	}

	data, err := io.ReadAll(buffer)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	// Configure the request with the WAV data, sample rate, and language
	req := &speechpb.RecognizeRequest{
		Config: &speechpb.RecognitionConfig{
			Encoding: speechpb.RecognitionConfig_LINEAR16,
			//SampleRateHertz: 48000, // Set the sample rate of your WAV file
			LanguageCode: "en-US",
		},
		Audio: &speechpb.RecognitionAudio{
			AudioSource: &speechpb.RecognitionAudio_Content{Content: data},
		},
	}

	// Send the request to the Speech-to-Text API
	resp, err := client.Recognize(ctx, req)
	if err != nil {
		log.Fatalf("Failed to recognize: %v", err)
	}

	var res string

	// Process and print the results
	for _, result := range resp.Results {
		for _, alt := range result.Alternatives {
			fmt.Printf("Transcript: %s\n", alt.Transcript)
			res += alt.Transcript
		}
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": res})
}

func AiSpeechToTextWhisper(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		logrus.Warningf("%d: no requester", fiber.StatusForbidden)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		logrus.Warningf("%d: banned", fiber.StatusForbidden)
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	// Process uploaded file

	//region Process multipart form uploaded file

	// Get upload file
	formFile, err := c.FormFile("file")
	if err != nil {
		return err
	}

	// Get upload file buffer
	buffer, err := formFile.Open()
	if err != nil {
		return err
	}
	defer func(buffer multipart.File) {
		err := buffer.Close()
		if err != nil {
			logrus.Errorf("error closing multipart file: %v", err)
		}
	}(buffer)
	//endregion

	ctx := c.UserContext()

	// Get the AI client
	driver, ok := c.UserContext().Value(vContext.AI).(*ai.Driver)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "ai features not enabled", "data": nil})
	}

	data, err := io.ReadAll(buffer)
	if err != nil {
		log.Fatalf("Failed to read file: %v", err)
	}

	// Generate random uuid for file name
	fileUuid, err := uuid.NewV4()
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "error generating uuid", "data": nil})
	}

	err = os.WriteFile(fileUuid.String()+".wav", data, 0644)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "error writing file", "data": nil})
	}

	resp, err := driver.OpenAiClient.CreateTranscription(ctx, openai.AudioRequest{
		Model:    "whisper-1",
		FilePath: fileUuid.String() + ".wav",
		Language: "en",
	})

	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "error creating transcription", "data": nil})
	}

	return c.Status(fiber.StatusOK).JSON(fiber.Map{"status": "ok", "message": "ok", "data": resp.Text})
}

func CognitiveAi(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	// Get the AI client
	driver, ok := c.UserContext().Value(vContext.AI).(*ai.Driver)
	if !ok {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "ai features not enabled", "data": nil})
	}

	// Parse the request body
	var m struct {
		Data []ai.ChatHistoryMessage `json:"data"`
	}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	res, err := driver.Request(c.UserContext(), m.Data, "")
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Return the ai perception
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": res})
}

func CognitiveAiUser(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	// Parse the request body
	var m struct {
		Key     string                  `json:"key"`
		Version string                  `json:"version"`
		Data    []ai.ChatHistoryMessage `json:"data"`
	}
	err = c.BodyParser(&m)
	if err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Get the AI client
	driver := ai.MakeDriver(m.Key)
	if driver == nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": "failed to setup OpenAI client", "data": nil})
	}

	res, err := driver.Request(c.UserContext(), m.Data, m.Version)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"status": "error", "message": err.Error(), "data": nil})
	}

	// Return the ai perception
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": res})
}

func GetAiPerception(c *fiber.Ctx) error {
	//region Requester

	// Get requester
	requester, err := helper.GetRequester(c)
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "no requester", "data": nil})
	}

	// Check if requester is banned
	if requester.IsBanned {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"status": "error", "message": "banned", "data": nil})
	}

	//endregion

	// Return the ai perception
	return c.Status(fiber.StatusOK).JSON(fiber.Map{"data": "[]"})
}
