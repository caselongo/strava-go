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

// An OAuthAuthenticator holds state about how OAuth requests should be authenticated.
type OAuthAuthenticator struct {
	tokenSource TokenSource

	callbackUrl string // used to help generate the AuthorizationURL

	// The requestClientGenerator builds the http.Client that will be used
	// to complete the token exchange. If nil, http.DefaultClient will be used.
	// On Google's App Engine http.DefaultClient is not available and this generator
	// can be used to create a client using the incoming request, for Example:
	//    func(r *http.Request) { return urlfetch.Client(appengine.NewContext(r)) }
	requestClientGenerator func(r *http.Request) *http.Client
}

// NewOAuthAuthenticator creates a new OAuthAuthenticator instance.
func NewOAuthAuthenticator(tokenSource TokenSource, callbackUrl string) (*OAuthAuthenticator, error) {
	return &OAuthAuthenticator{
		tokenSource: tokenSource,
		callbackUrl: callbackUrl,
	}, nil
}

// Scope represents the access of an access_token.
// The scope type is requested during the token exchange.
type Scope string

const (
	ScopeRead            Scope = "read"
	ScopeReadAll         Scope = "read_all"
	ScopeProfileReadAll  Scope = "profile:read_all"
	ScopeProfileWrite    Scope = "profile:write"
	ScopeActivityRead    Scope = "activity:read"
	ScopeActivityReadAll Scope = "activity:read_all"
	ScopeActivityWrite   Scope = "activity:write"
)

// AuthorizationResponse is returned as a result of the token exchange
type AuthorizationResponse struct {
	TokenType    string           `json:"token_type"`
	ExpiresAt    int64            `json:"expires_at"`
	ExpiresIn    int64            `json:"expires_in"`
	RefreshToken string           `json:"refresh_token"`
	AccessToken  string           `json:"access_token"`
	State        string           `json:"state,omitempty"`
	Athlete      *AthleteDetailed `json:"athlete,omitempty"`
}

// CallbackPath returns the path portion of the callbackUrl.
// Useful when setting a http path handler, for example:
//
//	http.HandleFunc(stravaOAuth.callbackUrl(), stravaOAuth.HandlerFunc(successCallback, failureCallback))
func (auth OAuthAuthenticator) CallbackPath() (string, error) {
	if auth.callbackUrl == "" {
		return "", errors.New("callbackURL is empty")
	}
	callbackUrl, err := url.Parse(auth.callbackUrl)
	if err != nil {
		return "", err
	}
	return callbackUrl.Path, nil
}

// Authorize performs the second part of the OAuth exchange. The client has already been redirected to the
// Strava authorization page, has granted authorization to the application and has been redirected back to the
// defined URL. The code param was returned as a query string param in to the redirect_url.
func (auth OAuthAuthenticator) Authorize(code string, state string, client *http.Client) error {
	// make sure a code was passed
	if code == "" {
		return OAuthInvalidCodeErr
	}

	// if a client wasn't passed use the default client
	if client == nil {
		client = http.DefaultClient
	}

	resp, err := client.PostForm(basePath+"/oauth/token",
		url.Values{"client_id": {fmt.Sprintf("%d", ClientId)}, "client_secret": {ClientSecret}, "code": {code}})

	// this was a poor request, maybe strava servers down?
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	// check status code, could be 500, or most likely the client_secret is incorrect
	if resp.StatusCode/100 == 5 {
		return OAuthServerErr
	}

	if resp.StatusCode/100 != 2 {
		var response Error
		contents, _ := io.ReadAll(resp.Body)
		err = json.Unmarshal(contents, &response)
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

	response.State = state

	return auth.tokenSource.SaveAuthorizationResponse(&response)
}

// HandlerFunc builds a http.HandlerFunc that will complete the token exchange
// after a user authorizes an application on strava.com.
// This method handles the exchange and calls success or failure after it completes.
func (auth OAuthAuthenticator) HandlerFunc(
	success func(w http.ResponseWriter, r *http.Request),
	failure func(err error, w http.ResponseWriter, r *http.Request)) http.HandlerFunc {

	return func(w http.ResponseWriter, r *http.Request) {
		// user denied authorization
		if r.FormValue("error") == "access_denied" {
			failure(OAuthAuthorizationDeniedErr, w, r)
			return
		}

		// use the client generator if provided.
		client := http.DefaultClient
		if auth.requestClientGenerator != nil {
			client = auth.requestClientGenerator(r)
		}

		err := auth.Authorize(r.FormValue("code"), r.FormValue("state"), client)
		if err != nil {
			failure(err, w, r)
			return
		}
		success(w, r)
	}
}

// AuthorizationURL constructs the url a user should use to authorize this specific application.
func (auth OAuthAuthenticator) AuthorizationURL(state string, scopes []Scope, force bool) string {
	var s []string
	for _, scope := range scopes {
		s = append(s, string(scope))
	}

	path := fmt.Sprintf("%s/oauth/authorize?client_id=%d&response_type=code&redirect_uri=%s&scope=%v", basePath, ClientId, auth.callbackUrl, strings.Join(s, ","))

	if state != "" {
		path += "&state=" + state
	}

	if force {
		path += "&approval_prompt=force"
	}

	return path
}

/*********************************************************/

type OAuthService struct {
	client *Client
}

func NewOAuthService(client *Client) *OAuthService {
	return &OAuthService{client}
}

type OAuthDeauthorizeCall struct {
	service *OAuthService
}

func (s *OAuthService) Deauthorize() *OAuthDeauthorizeCall {
	return &OAuthDeauthorizeCall{
		service: s,
	}
}

func (c *OAuthDeauthorizeCall) Do() error {
	_, err := c.service.client.run("POST", "/oauth/deauthorize", nil)
	return err
}
