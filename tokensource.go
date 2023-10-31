package strava

type TokenSource interface {
	GetAuthorizationResponse() (*AuthorizationResponse, error)
	SaveAuthorizationResponse(string, *AuthorizationResponse) error
}
