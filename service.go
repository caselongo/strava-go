package strava_go

import (
	"errors"
	"net/http"
)

const (
	authorizeUrl string = "https://www.strava.com/oauth/authorize"
)

type Service struct {
	clientId     int
	clientSecret string
	httpClient   *http.Client
}

type ServiceConfig struct {
	ClientId     int
	ClientSecret string
}

func NewService(serviceConfig *ServiceConfig) (*Service, error) {
	if serviceConfig == nil {
		return nil, errors.New("serviceConfig is nil pointer")
	}

	return &Service{
		clientId:     serviceConfig.ClientId,
		clientSecret: serviceConfig.ClientSecret,
		httpClient:   &http.Client{},
	}, nil
}

func (service *Service) Authorize() {

}
