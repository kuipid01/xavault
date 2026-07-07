package cloudinary

import (
	"bytes"
	"crypto/sha1"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"os"
	"sort"
	"strings"
	"time"
)

type UploadResult struct {
	PublicID  string `json:"public_id"`
	SecureURL string `json:"secure_url"`
}

type Service struct {
	cloudName string
	apiKey    string
	apiSecret string
}

func NewService() *Service {
	return &Service{
		cloudName: strings.TrimSpace(os.Getenv("CLOUDINARY_CLOUD_NAME")),
		apiKey:    strings.TrimSpace(os.Getenv("CLOUDINARY_API_KEY")),
		apiSecret: strings.TrimSpace(os.Getenv("CLOUDINARY_API_SECRET")),
	}
}

func (s *Service) UploadImage(file multipart.File, filename string, folder string) (*UploadResult, error) {
	if s.cloudName == "" || s.apiKey == "" || s.apiSecret == "" {
		return nil, fmt.Errorf("cloudinary is not configured")
	}

	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", filename)
	if err != nil {
		return nil, err
	}
	if _, err := io.Copy(part, file); err != nil {
		return nil, err
	}

	timestamp := fmt.Sprintf("%d", time.Now().Unix())
	params := map[string]string{
		"folder":    folder,
		"timestamp": timestamp,
	}

	_ = writer.WriteField("api_key", s.apiKey)
	_ = writer.WriteField("folder", folder)
	_ = writer.WriteField("timestamp", timestamp)
	_ = writer.WriteField("signature", s.signature(params))

	if err := writer.Close(); err != nil {
		return nil, err
	}

	url := fmt.Sprintf("https://api.cloudinary.com/v1_1/%s/image/upload", s.cloudName)
	req, err := http.NewRequest(http.MethodPost, url, &body)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())

	res, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()

	resBody, _ := io.ReadAll(res.Body)
	if res.StatusCode < 200 || res.StatusCode >= 300 {
		return nil, fmt.Errorf("cloudinary upload failed: %s", string(resBody))
	}

	var result UploadResult
	if err := json.Unmarshal(resBody, &result); err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *Service) signature(params map[string]string) string {
	keys := make([]string, 0, len(params))
	for key := range params {
		keys = append(keys, key)
	}
	sort.Strings(keys)

	parts := make([]string, 0, len(keys))
	for _, key := range keys {
		parts = append(parts, key+"="+params[key])
	}

	sum := sha1.Sum([]byte(strings.Join(parts, "&") + s.apiSecret))
	return hex.EncodeToString(sum[:])
}
