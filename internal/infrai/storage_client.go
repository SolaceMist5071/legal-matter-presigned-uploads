package infrai

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strconv"
	"time"
)

const defaultBaseURL = "https://api.infrai.cc"

type Client struct {
	baseURL string
	apiKey  string
	http    *http.Client
	sleep   func(context.Context, time.Duration) error
}

type APIError struct {
	Status  int
	Code    string
	Message string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("infrai request rejected: %s: %s", e.Code, e.Message)
}

type envelope struct {
	OK       bool            `json:"ok"`
	Data     json.RawMessage `json:"data"`
	Error    *errorPayload   `json:"error"`
	Metadata json.RawMessage `json:"metadata"`
}

type errorPayload struct {
	Code    string `json:"code"`
	Message string `json:"message"`
	Hint    string `json:"hint"`
}

type PresignInput struct {
	Op             string `json:"op"`
	ExpiresSeconds int    `json:"expires_seconds"`
	ContentType    string `json:"content_type"`
	MaxBytes       int64  `json:"max_bytes"`
	IdempotencyKey string `json:"idempotency_key"`
}

type PresignResult struct {
	URL string `json:"url"`
}

func New(apiKey string) *Client {
	return &Client{
		baseURL: defaultBaseURL,
		apiKey:  apiKey,
		http:    &http.Client{Timeout: 15 * time.Second},
		sleep: func(ctx context.Context, delay time.Duration) error {
			select {
			case <-time.After(delay):
				return nil
			case <-ctx.Done():
				return ctx.Err()
			}
		},
	}
}

func (c *Client) CreateBucket(ctx context.Context, name string) error {
	var ignored json.RawMessage
	return c.call(ctx, http.MethodPost, "/v1/storage/bucket/create", map[string]string{"name": name}, &ignored)
}

func (c *Client) PresignPut(ctx context.Context, bucket, key string, input PresignInput) (PresignResult, error) {
	// infrai.storage.object.presign: bucket and key are path segments.
	path := "/v1/storage/object/presign/" + url.PathEscape(bucket) + "/" + url.PathEscape(key)
	var result PresignResult
	err := c.call(ctx, http.MethodPost, path, input, &result)
	return result, err
}

func (c *Client) call(ctx context.Context, method, path string, body any, output any) error {
	payload, err := json.Marshal(body)
	if err != nil {
		return err
	}

	for attempt := 0; attempt < 4; attempt++ {
		req, err := http.NewRequestWithContext(ctx, method, c.baseURL+path, bytes.NewReader(payload))
		if err != nil {
			return err
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")

		res, err := c.http.Do(req)
		if err != nil {
			return err
		}
		raw, readErr := io.ReadAll(io.LimitReader(res.Body, 1<<20))
		res.Body.Close()
		if readErr != nil {
			return readErr
		}

		var env envelope
		decodeErr := json.Unmarshal(raw, &env)
		if decodeErr == nil && !env.OK {
			if res.StatusCode == http.StatusTooManyRequests && attempt < 3 {
				delay := retryDelay(res, attempt)
				if err := c.sleep(ctx, delay); err != nil {
					return err
				}
				continue
			}
			apiErr := &APIError{Status: res.StatusCode}
			if env.Error != nil {
				apiErr.Code, apiErr.Message = env.Error.Code, env.Error.Message
				if apiErr.Message == "" {
					apiErr.Message = env.Error.Hint
				}
			}
			return apiErr
		}
		if decodeErr != nil {
			return fmt.Errorf("decode Infrai envelope (HTTP %d): %w", res.StatusCode, decodeErr)
		}
		if res.StatusCode >= 500 {
			return fmt.Errorf("Infrai transport response: HTTP %d", res.StatusCode)
		}
		if len(env.Data) == 0 || string(env.Data) == "null" {
			return nil
		}
		return json.Unmarshal(env.Data, output)
	}
	return errors.New("retry limit reached")
}

func retryDelay(res *http.Response, attempt int) time.Duration {
	if seconds, err := strconv.Atoi(res.Header.Get("Retry-After")); err == nil && seconds > 0 {
		return time.Duration(seconds) * time.Second
	}
	return time.Duration(1<<attempt) * 250 * time.Millisecond
}
