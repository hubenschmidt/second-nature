package audio

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"strings"
)

type whisperResponse struct {
	Text string `json:"text"`
}

func transcribe(wavData []byte, whisperURL string) (string, error) {
	var body bytes.Buffer
	w := multipart.NewWriter(&body)

	part, err := w.CreateFormFile("file", "audio.wav")
	if err != nil {
		return "", fmt.Errorf("create form file: %w", err)
	}
	if _, err := part.Write(wavData); err != nil {
		return "", fmt.Errorf("write wav: %w", err)
	}

	w.WriteField("response_format", "json")
	w.WriteField("temperature", "0.0")
	w.Close()

	resp, err := http.Post(whisperURL+"/inference", w.FormDataContentType(), &body)
	if err != nil {
		return "", fmt.Errorf("whisper request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("whisper %d: %s", resp.StatusCode, string(b))
	}

	var result whisperResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}

	text := strings.TrimSpace(result.Text)
	if text == "" {
		return "", fmt.Errorf("no speech detected")
	}
	return text, nil
}
