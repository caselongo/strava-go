package strava

type TokenSource interface {
	GetAuthorizationResponse() (*AuthorizationResponse, error)
	SaveAuthorizationResponse(*AuthorizationResponse) error
}
