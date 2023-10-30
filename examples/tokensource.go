package main

import "github.com/caselongo/strava-go"

type StaticTokenSource struct{}

func (tokenSource *StaticTokenSource) GetAuthorizationResponse() (*strava.AuthorizationResponse, error) {
	return &strava.AuthorizationResponse{
		AccessToken: "YOUR STATIC ACCESSTOKEN",
	}, nil
}

func (tokenSource *StaticTokenSource) SaveAuthorizationResponse(a *strava.AuthorizationResponse) error {
	return nil
}
