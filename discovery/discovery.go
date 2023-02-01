package discovery

import (
	"fmt"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/rs/zerolog/log"
	"os"
)

const serviceName = "generator"

func RegisterInConsul(port int) (*consulapi.Agent, string) {

	config := consulapi.DefaultConfig()
	config.Address = os.Getenv("consul_server_address")
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
	return consul.Agent(), serviceID
}
