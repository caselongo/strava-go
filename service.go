package strava

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
)

var ClientId int
var ClientSecret string

const basePath = "https://www.strava.com/api/v3"
const timeFormat = "2006-01-02T15:04:05Z"

type Client struct {
	tokenSource           TokenSource
	authorizationResponse *AuthorizationResponse
	httpClient            *http.Client
}

type ErrorHandler func(*http.Response) error

var defaultErrorHandler ErrorHandler = func(resp *http.Response) error {
	// check status code, could be 500, or most likely the client_secret is incorrect
	if resp.StatusCode/100 == 5 {
		return errors.New("server error")
	}

	if resp.StatusCode/100 == 4 {
		var response Error
		contents, _ := io.ReadAll(resp.Body)
		err := json.Unmarshal(contents, &response)
		if err != nil {
			return err
		}

		return response
	}

	if resp.StatusCode/100 == 3 {
		return errors.New("redirect error")
	}
	return nil
}

// validateToken validates the current token provided by TokenSource.
// if retrieves the token if it not already did and refreshes the token if it has expired
func (client *Client) validateToken() error {
	if client.authorizationResponse == nil {
		authorizationResponse, err := client.tokenSource.GetAuthorizationResponse()
		if err != nil {
			return err
		}

		client.authorizationResponse = authorizationResponse
	}

	if client.authorizationResponse.AccessToken == "" {
		return errors.New("accesstoken is empty string")
	}

	/*
		expiresAt := time.UnixMicro(client.authorizationResponse.ExpiresAt * 1000)
		if expiresAt.After(time.Now().Add(10 * time.Second)) {
			return nil
		}*/

	return client.refreshToken()
}

// refreshToken refreshes the token if it has expired
func (client *Client) refreshToken() error {
	values := make(url.Values)
	values.Set("client_id", fmt.Sprintf("%d", ClientId))
	values.Set("client_secret", ClientSecret)
	values.Set("grant_type", "refresh_token")
	values.Set("refresh_token", client.authorizationResponse.RefreshToken)

	fmt.Println(basePath+"/oauth/token", values)
	resp, err := client.httpClient.PostForm(basePath+"/oauth/token", values)
	if err != nil {
		return err
	}

	// check status code, could be 500, or most likely the client_secret is incorrect
	if resp.StatusCode/100 == 5 {
		return OAuthServerErr
	}

	if resp.StatusCode/100 != 2 {
		var response Error
		contents, _ := io.ReadAll(resp.Body)
		err := json.Unmarshal(contents, &response)
		if err != nil {
			return err
		}

		if len(response.Errors) == 0 {
			return OAuthServerErr
		}

		if response.Errors[0].Resource == "Application" {
			return OAuthInvalidCredentialsErr
		}

		if response.Errors[0].Resource == "RequestToken" {
			return OAuthInvalidCodeErr
		}

		return &response
	}

	var response AuthorizationResponse
	contents, _ := io.ReadAll(resp.Body)
	err = json.Unmarshal(contents, &response)
	if err != nil {
		return err
	}

	client.authorizationResponse = &response

	err = client.tokenSource.SaveAuthorizationResponse(&response)
	if err != nil {
		return err
	}

	return nil
}

// NewClient builds a normal client for making requests to the strava api.
// a http.Client can be passed in if http.DefaultClient can not be used.
func NewClient(tokenSource TokenSource, client ...*http.Client) *Client {
	c := &Client{tokenSource: tokenSource}
	if len(client) != 0 {
		c.httpClient = client[0]
	} else {
		c.httpClient = http.DefaultClient
	}
	return c
}

// NewStubResponseClient can be used for testing
// TODO, stub out with an actual response
func NewStubResponseClient(content string, statusCode ...int) *Client {
	c := NewClient(nil)
	t := &stubResponseTransport{content: content}

	if len(statusCode) != 0 {
		t.statusCode = statusCode[0]
	}

	c.httpClient = &http.Client{Transport: t}

	return c
}

type stubResponseTransport struct {
	http.Transport
	content    string
	statusCode int
}

func (t *stubResponseTransport) RoundTrip(req *http.Request) (*http.Response, error) {
	resp := &http.Response{
		Status:     http.StatusText(t.statusCode),
		StatusCode: t.statusCode,
	}
	resp.Body = io.NopCloser(strings.NewReader(t.content))

	return resp, nil
}

func (client *Client) run(method, path string, params map[string]interface{}) ([]byte, error) {
	var err error

	values := make(url.Values)
	for k, v := range params {
		values.Set(k, fmt.Sprintf("%v", v))
	}

	var req *http.Request
	if method == "POST" {
		req, err = http.NewRequest("POST", basePath+path, strings.NewReader(values.Encode()))
		if err != nil {
			return nil, err
		}
		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	} else {
		req, err = http.NewRequest(method, basePath+path+"?"+values.Encode(), nil)
		if err != nil {
			return nil, err
		}
	}

	return client.runRequest(req)
}

func (client *Client) runRequestWithErrorHandler(req *http.Request, errorHandler ErrorHandler) ([]byte, error) {
	err := client.validateToken()
	if err != nil {
		return nil, err
	}

	req.Header.Set("Authorization", "Bearer "+client.authorizationResponse.AccessToken)
	req.Header.Set("User-Agent", "caselongo/strava-go")
	resp, err := client.httpClient.Do(req)

	// this was a poor request, maybe strava servers down?
	if err != nil {
		return nil, err
	}

	defer resp.Body.Close()

	RateLimiting.updateRateLimits(resp)

	return checkResponseForErrorsWithErrorHandler(resp, errorHandler)
}

func (client *Client) runRequest(req *http.Request) ([]byte, error) {
	return client.runRequestWithErrorHandler(req, defaultErrorHandler)
}

func checkResponseForErrorsWithErrorHandler(resp *http.Response, errorHandler ErrorHandler) ([]byte, error) {
	if resp.StatusCode/100 > 2 {
		return nil, errorHandler(resp)
	} else {
		return io.ReadAll(resp.Body)
	}
}

func checkResponseForErrors(resp *http.Response) ([]byte, error) {
	return checkResponseForErrorsWithErrorHandler(resp, defaultErrorHandler)
}
