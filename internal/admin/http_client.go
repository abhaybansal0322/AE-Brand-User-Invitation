package admin

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/abhaybansal0322/AE-Brand-User-Invitation/internal/domain"
)

type HTTPClient struct {
	baseURL    *url.URL
	httpClient *http.Client
}

func NewHTTPClient(baseURL string, httpClient *http.Client) (*HTTPClient, error) {
	parsed, err := url.Parse(strings.TrimSpace(baseURL))
	if err != nil || parsed.Scheme == "" || parsed.Host == "" {
		return nil, fmt.Errorf("%w: invalid admin api base url", domain.ErrInvalidArgument)
	}
	if httpClient == nil {
		httpClient = &http.Client{Timeout: 2 * time.Second}
	}
	return &HTTPClient{baseURL: parsed, httpClient: httpClient}, nil
}

func (c *HTTPClient) CreateUser(ctx context.Context, user domain.UserDetails) (domain.UserDetails, error) {
	var response domain.UserDetails
	err := c.doJSON(ctx, http.MethodPost, "/users", user, &response)
	if err != nil {
		return domain.UserDetails{}, err
	}
	return response, nil
}

func (c *HTTPClient) GetUserByEmail(ctx context.Context, email string) (domain.UserDetails, error) {
	values := url.Values{"email": []string{email}}
	var response domain.UserDetails
	err := c.doJSON(ctx, http.MethodGet, "/users/by-email?"+values.Encode(), nil, &response)
	if err != nil {
		return domain.UserDetails{}, err
	}
	return response, nil
}

func (c *HTTPClient) GetUserByID(ctx context.Context, userPool, userID string) (domain.UserDetails, error) {
	values := url.Values{"user_pool": []string{userPool}}
	var response domain.UserDetails
	err := c.doJSON(ctx, http.MethodGet, "/users/"+url.PathEscape(userID)+"?"+values.Encode(), nil, &response)
	if err != nil {
		return domain.UserDetails{}, err
	}
	return response, nil
}

func (c *HTTPClient) doJSON(ctx context.Context, method, path string, requestBody any, responseBody any) error {
	endpoint := c.baseURL.ResolveReference(&url.URL{Path: strings.TrimRight(c.baseURL.Path, "/") + strings.Split(path, "?")[0]})
	if strings.Contains(path, "?") {
		endpoint.RawQuery = strings.SplitN(path, "?", 2)[1]
	}

	var body *bytes.Reader
	if requestBody != nil {
		bytesBody, err := json.Marshal(requestBody)
		if err != nil {
			return fmt.Errorf("%w: encode admin request", domain.ErrInvalidArgument)
		}
		body = bytes.NewReader(bytesBody)
	} else {
		body = bytes.NewReader(nil)
	}

	req, err := http.NewRequestWithContext(ctx, method, endpoint.String(), body)
	if err != nil {
		return err
	}
	req.Header.Set("Accept", "application/json")
	if requestBody != nil {
		req.Header.Set("Content-Type", "application/json")
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		if errors.Is(ctx.Err(), context.DeadlineExceeded) || errors.Is(ctx.Err(), context.Canceled) {
			return ctx.Err()
		}
		return fmt.Errorf("%w: admin api request failed: %v", domain.ErrUnavailable, err)
	}
	defer resp.Body.Close()

	switch resp.StatusCode {
	case http.StatusOK, http.StatusCreated:
		if responseBody == nil {
			return nil
		}
		if err := json.NewDecoder(resp.Body).Decode(responseBody); err != nil {
			return fmt.Errorf("%w: decode admin response", domain.ErrUnavailable)
		}
		return nil
	case http.StatusConflict:
		return fmt.Errorf("%w: admin user already exists", domain.ErrAlreadyExists)
	case http.StatusNotFound:
		return fmt.Errorf("%w: admin user not found", domain.ErrNotFound)
	case http.StatusBadRequest:
		return fmt.Errorf("%w: admin rejected request", domain.ErrInvalidArgument)
	default:
		return fmt.Errorf("%w: admin api returned %d", domain.ErrUnavailable, resp.StatusCode)
	}
}
