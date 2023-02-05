package discovery

import (
	"bytes"
	"fmt"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/rs/zerolog/log"
	"net/http"
	"os"
)

const serviceName = "generator"

type Service struct {
	serviceId   string
	consulAgent *consulapi.Agent
}

func RegisterInConsul(port int) *Service {
	config := consulapi.DefaultConfig()
	config.Address = os.Getenv("consul_server_address")
	if config.Address == "" {
		log.Info().Msg("Skip registration in consul")
		return nil
	}
	consul, err := consulapi.NewClient(config)

	if err != nil {
		log.Fatal().Err(err)
	}

	address := os.Getenv("HOSTNAME")
	serviceID := fmt.Sprintf("%s-%s-%d", serviceName, address, port)

	registration := &consulapi.AgentServiceRegistration{
		ID:   serviceID,
		Name: serviceName,
		Port: port,
		Check: &consulapi.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%d/health", address, port),
			Interval: "15s",
			Timeout:  "20s",
		},
		Tags: []string{"prometheus_monitoring_endpoint=/metrics"},
	}

	if address != "" {
		registration.Address = address
	}

	regiErr := consul.Agent().ServiceRegister(registration)

	if regiErr != nil {
		log.Fatal().Err(regiErr).Msgf("Failed to register service: %s:%v", address, port)
	} else {
		log.Info().Msgf("Successfully register service: %s:%v", address, port)
	}
	return &Service{serviceId: serviceID, consulAgent: consul.Agent()}
}

func (service *Service) SendEndTestRequestToMain(testId string) {
	mainService, _, err := service.consulAgent.Service("ledokol-main", &consulapi.QueryOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to find ledokol-main")
		return
	}

	address := mainService.Address
	port := mainService.Port
	url := fmt.Sprintf("http://%s:%d/api/testruns/%s/end", address, port, testId)
	response, err := http.Post(url, "application/json", &bytes.Buffer{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to send end test request to main component")
		return
	}
	if response.StatusCode != 200 {
		log.Info().Msgf("Status %d when send end test request to main component", response.StatusCode)
		return
	}

	log.Info().Msg("Successfully sent end test request to main component")
}

func (service *Service) DeregisterInConsul() {
	service.consulAgent.ServiceDeregister(service.serviceId)
	log.Info().Msg("Service deregistered")
}
