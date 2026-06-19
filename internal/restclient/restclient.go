package restclient

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

var (
	ErrTemplateNotFound     = errors.New("restclient: template not found")
	ErrTemplateNameEmpty    = errors.New("restclient: template name is empty")
	ErrPathParamMissing     = errors.New("restclient: missing path parameter")
	ErrRequestBuildFailed   = errors.New("restclient: failed to build request")
	ErrMaxRetriesExceeded   = errors.New("restclient: max retries exceeded")
	ErrAuthProviderNotFound = errors.New("restclient: auth provider not found")
)

type requestBuildError struct {
	err error
}

func (e *requestBuildError) Error() string {
	return ErrRequestBuildFailed.Error() + ": " + e.err.Error()
}

func (e *requestBuildError) Unwrap() error {
	return e.err
}

func (e *requestBuildError) Is(target error) bool {
	return target == ErrRequestBuildFailed
}

type AuthProvider interface {
	Name() string
	Inject(req *http.Request) error
}

type RequestTemplate struct {
	Name           string
	Method         string
	BaseURL        string
	Path           string
	DefaultHeaders http.Header
	Timeout        time.Duration
	MaxRetries     int
	RetryInterval  time.Duration
	AuthProvider   string
}

type RequestOptions struct {
	PathParams  map[string]string
	QueryParams map[string]string
	Headers     http.Header
	Body        []byte
}

type Client struct {
	mu            sync.RWMutex
	templates     map[string]*RequestTemplate
	authProviders map[string]AuthProvider
	httpClient    *http.Client
	baseURL       string
}

type ClientOption func(*Client)

func WithBaseURL(baseURL string) ClientOption {
	return func(c *Client) {
		c.baseURL = baseURL
	}
}

func WithHTTPClient(hc *http.Client) ClientOption {
	return func(c *Client) {
		c.httpClient = hc
	}
}

func NewClient(opts ...ClientOption) *Client {
	c := &Client{
		templates:     make(map[string]*RequestTemplate),
		authProviders: make(map[string]AuthProvider),
		httpClient: &http.Client{
			Timeout: 30 * time.Second,
		},
	}
	for _, opt := range opts {
		opt(c)
	}
	return c
}

func (c *Client) RegisterTemplate(tmpl RequestTemplate) error {
	if tmpl.Name == "" {
		return ErrTemplateNameEmpty
	}
	if tmpl.Method == "" {
		tmpl.Method = http.MethodGet
	}
	if tmpl.DefaultHeaders == nil {
		tmpl.DefaultHeaders = make(http.Header)
	}
	if tmpl.Timeout < 0 {
		tmpl.Timeout = 0
	}
	if tmpl.MaxRetries < 0 {
		tmpl.MaxRetries = 0
	}
	if tmpl.RetryInterval < 0 {
		tmpl.RetryInterval = 0
	}

	c.mu.Lock()
	defer c.mu.Unlock()
	c.templates[tmpl.Name] = &tmpl
	return nil
}

func (c *Client) UnregisterTemplate(name string) {
	c.mu.Lock()
	defer c.mu.Unlock()
	delete(c.templates, name)
}

func (c *Client) GetTemplate(name string) (*RequestTemplate, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	tmpl, ok := c.templates[name]
	if !ok {
		return nil, ErrTemplateNotFound
	}
	tmplCopy := *tmpl
	return &tmplCopy, nil
}

func (c *Client) RegisterAuthProvider(provider AuthProvider) error {
	if provider == nil {
		return ErrAuthProviderNotFound
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	c.authProviders[provider.Name()] = provider
	return nil
}

func (c *Client) GetAuthProvider(name string) (AuthProvider, error) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	p, ok := c.authProviders[name]
	if !ok {
		return nil, ErrAuthProviderNotFound
	}
	return p, nil
}

func (c *Client) Do(ctx context.Context, templateName string, opts *RequestOptions) (*http.Response, error) {
	c.mu.RLock()
	tmpl, ok := c.templates[templateName]
	c.mu.RUnlock()
	if !ok {
		return nil, ErrTemplateNotFound
	}

	if opts == nil {
		opts = &RequestOptions{}
	}

	attempt := 0
	var lastErr error
	var lastResp *http.Response

	for {
		if ctx.Err() != nil {
			return nil, ctx.Err()
		}

		resp, err := c.doRequest(ctx, tmpl, opts)

		if err == nil {
			return resp, nil
		}

		if !isRetryableError(err) {
			if resp != nil && resp.Body != nil {
				resp.Body.Close()
			}
			return nil, err
		}

		lastErr = err
		if resp != nil {
			if resp.Body != nil {
				resp.Body.Close()
			}
			lastResp = resp
		}

		attempt++
		if attempt > tmpl.MaxRetries {
			break
		}

		if tmpl.RetryInterval > 0 {
			timer := time.NewTimer(tmpl.RetryInterval)
			select {
			case <-ctx.Done():
				timer.Stop()
				return nil, ctx.Err()
			case <-timer.C:
			}
		}
	}

	if lastResp != nil {
		lastResp.Body = io.NopCloser(strings.NewReader(""))
		return lastResp, fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
	}
	return nil, fmt.Errorf("%w: %v", ErrMaxRetriesExceeded, lastErr)
}

func isRetryableError(err error) bool {
	if errors.Is(err, ErrRequestBuildFailed) {
		return false
	}
	if errors.Is(err, ErrPathParamMissing) {
		return false
	}
	if errors.Is(err, ErrAuthProviderNotFound) {
		return false
	}
	if errors.Is(err, context.Canceled) {
		return false
	}
	if errors.Is(err, context.DeadlineExceeded) {
		return true
	}
	return true
}

func (c *Client) doRequest(ctx context.Context, tmpl *RequestTemplate, opts *RequestOptions) (*http.Response, error) {
	reqURL, err := c.buildURL(tmpl, opts)
	if err != nil {
		return nil, &requestBuildError{err: err}
	}

	var bodyReader io.Reader
	if opts.Body != nil && len(opts.Body) > 0 {
		bodyReader = bytes.NewReader(opts.Body)
	}

	reqCtx := ctx
	if tmpl.Timeout > 0 {
		var cancel context.CancelFunc
		reqCtx, cancel = context.WithTimeout(ctx, tmpl.Timeout)
		defer cancel()
	}

	req, err := http.NewRequestWithContext(reqCtx, tmpl.Method, reqURL, bodyReader)
	if err != nil {
		return nil, &requestBuildError{err: err}
	}

	mergeHeaders(req.Header, tmpl.DefaultHeaders)
	if opts.Headers != nil {
		mergeHeaders(req.Header, opts.Headers)
	}

	if tmpl.AuthProvider != "" {
		c.mu.RLock()
		authProvider, ok := c.authProviders[tmpl.AuthProvider]
		c.mu.RUnlock()
		if !ok {
			return nil, ErrAuthProviderNotFound
		}
		if err := authProvider.Inject(req); err != nil {
			return nil, fmt.Errorf("restclient: auth injection failed: %w", err)
		}
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, err
	}
	return resp, nil
}

func (c *Client) buildURL(tmpl *RequestTemplate, opts *RequestOptions) (string, error) {
	base := c.baseURL
	if tmpl.BaseURL != "" {
		base = tmpl.BaseURL
	}

	path := tmpl.Path
	if opts.PathParams != nil && len(opts.PathParams) > 0 {
		var err error
		path, err = replacePathParams(path, opts.PathParams)
		if err != nil {
			return "", err
		}
	} else if strings.Contains(path, "{") && strings.Contains(path, "}") {
		return "", fmt.Errorf("%w: path contains placeholders but no path params provided", ErrPathParamMissing)
	}

	var fullURL string
	if base != "" {
		if strings.HasSuffix(base, "/") && strings.HasPrefix(path, "/") {
			fullURL = base + path[1:]
		} else if !strings.HasSuffix(base, "/") && !strings.HasPrefix(path, "/") {
			fullURL = base + "/" + path
		} else {
			fullURL = base + path
		}
	} else {
		fullURL = path
	}

	if opts.QueryParams != nil && len(opts.QueryParams) > 0 {
		query := url.Values{}
		for k, v := range opts.QueryParams {
			query.Set(k, v)
		}
		queryStr := query.Encode()
		if queryStr != "" {
			if strings.Contains(fullURL, "?") {
				fullURL += "&" + queryStr
			} else {
				fullURL += "?" + queryStr
			}
		}
	}

	return fullURL, nil
}

func replacePathParams(path string, params map[string]string) (string, error) {
	result := path
	for key, value := range params {
		placeholder := "{" + key + "}"
		result = strings.ReplaceAll(result, placeholder, url.PathEscape(value))
	}

	if strings.Contains(result, "{") && strings.Contains(result, "}") {
		start := strings.Index(result, "{")
		end := strings.Index(result, "}")
		if end > start {
			missing := result[start : end+1]
			return "", fmt.Errorf("%w: %s", ErrPathParamMissing, missing)
		}
	}

	return result, nil
}

func mergeHeaders(dst, src http.Header) {
	for key, values := range src {
		for _, v := range values {
			dst.Add(key, v)
		}
	}
}
