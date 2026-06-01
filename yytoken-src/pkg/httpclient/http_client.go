package httpclient

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"time"
)

// HttpClient HTTP客户端
type HttpClient struct {
	client  *http.Client
	baseURL string
	timeout time.Duration
}

// NewHttpClient 创建HTTP客户端
func NewHttpClient(baseURL string, timeout time.Duration) *HttpClient {
	return &HttpClient{
		client: &http.Client{
			Timeout: timeout,
		},
		baseURL: baseURL,
		timeout: timeout,
	}
}

// Get 发送GET请求
func (c *HttpClient) Get(ctx context.Context, path string, params map[string]string) (*http.Response, error) {
	url := c.baseURL + path

	// 添加查询参数
	if len(params) > 0 {
		url += "?"
		for key, value := range params {
			url += fmt.Sprintf("%s=%s&", key, value)
		}
		url = url[:len(url)-1] // 去掉最后的&
	}

	req, err := http.NewRequestWithContext(ctx, "GET", url, nil)
	if err != nil {
		return nil, err
	}

	return c.client.Do(req)
}

// Post 发送POST请求
func (c *HttpClient) Post(ctx context.Context, path string, data interface{}) (*http.Response, error) {
	url := c.baseURL + path

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return c.client.Do(req)
}

// PostForm 发送表单POST请求
func (c *HttpClient) PostForm(ctx context.Context, path string, data map[string]string) (*http.Response, error) {
	url := c.baseURL + path

	formData := make([]byte, 0)
	for key, value := range data {
		formData = append(formData, []byte(fmt.Sprintf("%s=%s&", key, value))...)
	}
	if len(formData) > 0 {
		formData = formData[:len(formData)-1] // 去掉最后的&
	}

	req, err := http.NewRequestWithContext(ctx, "POST", url, bytes.NewBuffer(formData))
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	return c.client.Do(req)
}

// Put 发送PUT请求
func (c *HttpClient) Put(ctx context.Context, path string, data interface{}) (*http.Response, error) {
	url := c.baseURL + path

	var body io.Reader
	if data != nil {
		jsonData, err := json.Marshal(data)
		if err != nil {
			return nil, err
		}
		body = bytes.NewBuffer(jsonData)
	}

	req, err := http.NewRequestWithContext(ctx, "PUT", url, body)
	if err != nil {
		return nil, err
	}

	req.Header.Set("Content-Type", "application/json")

	return c.client.Do(req)
}

// Delete 发送DELETE请求
func (c *HttpClient) Delete(ctx context.Context, path string) (*http.Response, error) {
	url := c.baseURL + path

	req, err := http.NewRequestWithContext(ctx, "DELETE", url, nil)
	if err != nil {
		return nil, err
	}

	return c.client.Do(req)
}

// SetHeader 设置请求头
func (c *HttpClient) SetHeader(req *http.Request, key, value string) {
	req.Header.Set(key, value)
}

// SetHeaders 批量设置请求头
func (c *HttpClient) SetHeaders(req *http.Request, headers map[string]string) {
	for key, value := range headers {
		req.Header.Set(key, value)
	}
}

// ParseResponse 解析响应
func (c *HttpClient) ParseResponse(resp *http.Response, result interface{}) error {
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return err
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("HTTP error: %d, body: %s", resp.StatusCode, string(body))
	}

	if result != nil {
		return json.Unmarshal(body, result)
	}

	return nil
}

// GetWithRetry 带重试的GET请求
func (c *HttpClient) GetWithRetry(ctx context.Context, path string, params map[string]string, maxRetries int) (*http.Response, error) {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		resp, err := c.Get(ctx, path, params)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if i < maxRetries {
			time.Sleep(time.Duration(i+1) * time.Second) // 递增延迟
		}
	}

	return nil, lastErr
}

// PostWithRetry 带重试的POST请求
func (c *HttpClient) PostWithRetry(ctx context.Context, path string, data interface{}, maxRetries int) (*http.Response, error) {
	var lastErr error

	for i := 0; i <= maxRetries; i++ {
		resp, err := c.Post(ctx, path, data)
		if err == nil {
			return resp, nil
		}

		lastErr = err
		if i < maxRetries {
			time.Sleep(time.Duration(i+1) * time.Second) // 递增延迟
		}
	}

	return nil, lastErr
}
