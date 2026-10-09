package v1

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mk20mm/homeagent/internal/apperr"
)

// TranscribeResponse 对齐 openapi TranscribeResponse。
type transcribeResponse struct {
	Text        string  `json:"text"`
	DurationSec float64 `json:"duration_sec,omitempty"`
	Provider    string  `json:"provider,omitempty"`
}

// TranscribeVoice 处理 POST /voice/transcribe 语音转文字请求。
// 优先连接本地极客 whisper.cpp 服务（100% 本地免费）；未配置时安全降级。
func TranscribeVoice() gin.HandlerFunc {
	return func(c *gin.Context) {
		_, ok := memberIDFrom(c)
		if !ok {
			abortWith(c, apperr.New(apperr.CodePermission, "缺少成员身份", nil))
			return
		}

		// 限制请求体最大 10MB
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, 10<<20)

		fileHeader, err := c.FormFile("file")
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "未获取到音频文件", err))
			return
		}

		language := c.DefaultPostForm("language", "zh")
		prompt := c.PostForm("prompt")

		file, err := fileHeader.Open()
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "打开音频文件失败", err))
			return
		}
		defer file.Close()

		audioData, err := io.ReadAll(file)
		if err != nil {
			abortWith(c, apperr.New(apperr.CodeInternal, "读取音频数据失败", err))
			return
		}

		if len(audioData) == 0 {
			abortWith(c, apperr.New(apperr.CodeInvalidInput, "音频内容为空", nil))
			return
		}

		// 1. 尝试连接本地极客 whisper.cpp 实例 (默认 http://localhost:8081 或 LOCAL_WHISPER_URL)
		localWhisperURL := os.Getenv("LOCAL_WHISPER_URL")
		if localWhisperURL == "" {
			localWhisperURL = "http://localhost:8081"
		}

		ctx, cancel := context.WithTimeout(c.Request.Context(), 8*time.Second)
		defer cancel()

		text, err := forwardToWhisperServer(ctx, localWhisperURL, fileHeader.Filename, audioData, language, prompt)
		if err == nil && strings.TrimSpace(text) != "" {
			c.JSON(http.StatusOK, transcribeResponse{
				Text:        strings.TrimSpace(text),
				DurationSec: float64(len(audioData)) / 32000.0, // 估算
				Provider:    "local_whisper_cpp",
			})
			return
		}

		// 2. 检查是否有第三方 Whisper 兼容接口
		cloudKey := os.Getenv("WHISPER_API_KEY")
		cloudBase := os.Getenv("WHISPER_BASE_URL")
		if cloudKey != "" {
			if cloudBase == "" {
				cloudBase = "https://api.openai.com/v1"
			}
			cloudText, err := forwardToOpenAIWhisper(ctx, cloudBase, cloudKey, fileHeader.Filename, audioData, language, prompt)
			if err == nil && strings.TrimSpace(cloudText) != "" {
				c.JSON(http.StatusOK, transcribeResponse{
					Text:        strings.TrimSpace(cloudText),
					DurationSec: float64(len(audioData)) / 32000.0,
					Provider:    "cloud_whisper",
				})
				return
			}
		}

		// 3. 本地与云端服务均未开启时的兜底提示 (引导使用浏览器内置免费识别)
		c.JSON(http.StatusOK, transcribeResponse{
			Text:        "已接收语音（推荐使用浏览器内置 Web Speech 免费实时识别）",
			DurationSec: float64(len(audioData)) / 32000.0,
			Provider:    "fallback_audio_receiver",
		})
	}
}

// forwardToWhisperServer 向本地 whisper.cpp 兼容服务转发音频。
func forwardToWhisperServer(ctx context.Context, serverURL, filename string, audio []byte, language, prompt string) (string, error) {
	parsed, err := url.Parse(serverURL)
	if err != nil {
		return "", err
	}

	endpoint := strings.TrimRight(parsed.String(), "/") + "/inference"

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}

	_ = writer.WriteField("language", language)
	if prompt != "" {
		_ = writer.WriteField("prompt", prompt)
	}
	_ = writer.WriteField("response_format", "json")
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	client := &http.Client{Timeout: 6 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("local whisper error status: %d", resp.StatusCode)
	}

	var res struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Text, nil
}

// forwardToOpenAIWhisper 转发到兼容 OpenAI 的云端 Whisper API。
func forwardToOpenAIWhisper(ctx context.Context, baseURL, apiKey, filename string, audio []byte, language, prompt string) (string, error) {
	endpoint := strings.TrimRight(baseURL, "/") + "/audio/transcriptions"

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return "", err
	}
	if _, err := part.Write(audio); err != nil {
		return "", err
	}

	_ = writer.WriteField("model", "whisper-1")
	_ = writer.WriteField("language", language)
	if prompt != "" {
		_ = writer.WriteField("prompt", prompt)
	}
	_ = writer.Close()

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, endpoint, &body)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+apiKey)

	client := &http.Client{Timeout: 7 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cloud whisper error status: %d", resp.StatusCode)
	}

	var res struct {
		Text string `json:"text"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return "", err
	}
	return res.Text, nil
}
