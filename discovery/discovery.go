package discovery

import (
	"bytes"
	"fmt"
	"net/http"

	consulapi "github.com/hashicorp/consul/api"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
)

const defaultServiceName = "generator"

type Service struct {
	serviceId     string
	consulAgent   *consulapi.Agent
	mainServiceId string
}

func RegisterInConsul(port int) *Service {
	config := consulapi.DefaultConfig()
	_ = viper.BindEnv("consul.address", "consul_server_address")
	config.Address = viper.GetString("consul.address")
	if config.Address == "" {
		log.Info().Msg("Skip registration in consul")
		return nil
	}
	consul, err := consulapi.NewClient(config)

	if err != nil {
		log.Fatal().Err(err).Msg("")
	}

	_ = viper.BindEnv("consul.generator-hostname", "HOSTNAME")
	address := viper.GetString("consul.generator-hostname")
	viper.SetDefault("consul.service-name", defaultServiceName)
	serviceName := viper.GetString("consul.service-name")
	serviceID := fmt.Sprintf("%s-%s-%d", serviceName, address, port)

	viper.SetDefault("consul.check.interval", "15s")
	viper.SetDefault("consul.check.timeout", "10s")

	registration := &consulapi.AgentServiceRegistration{
		ID:   serviceID,
		Name: serviceName,
		Port: port,
		Check: &consulapi.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%d/health", address, port),
			Interval: viper.GetString("consul.check.interval"),
			Timeout:  viper.GetString("consul.check.timeout"),
		},
		Tags: viper.GetStringSlice("consul.tags"),
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

	viper.SetDefault("consul.main-service-id", "ledokol-main")

	return &Service{serviceId: serviceID,
		consulAgent:   consul.Agent(),
		mainServiceId: viper.GetString("consul.main-service-id")}
}

func (service *Service) SendEndTestRequestToMain(testId string) {
	mainService, _, err := service.consulAgent.Service(service.mainServiceId, &consulapi.QueryOptions{})
	if err != nil {
		log.Error().Err(err).Str("testRunId", testId).Msg("Failed to find ledokol-main")
		return
	}

	address := mainService.Address
	port := mainService.Port
	url := fmt.Sprintf("http://%s:%d/api/testruns/%s?action=END&generatorId=%s", address, port, testId, service.serviceId)
	response, err := http.Post(url, "application/json", &bytes.Buffer{})
	if err != nil {
		log.Error().Err(err).Str("testRunId", testId).Msg("Failed to send end test request to main component")
		return
	}
	if response.StatusCode != 200 {
		log.Error().Str("testRunId", testId).Msgf("Status %d when send end test request to main component", response.StatusCode)
		return
	}

	log.Info().Str("testRunId", testId).Msg("Successfully sent end test request to main component")
}

func (service *Service) DeregisterInConsul() {
	service.consulAgent.ServiceDeregister(service.serviceId)
	log.Info().Msg("Service deregistered")
}
