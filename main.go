package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"ledokol/load"
	"log"
	"net/http"
	"os"
)

func main() {

	serverLogFile, _ := os.Create("server.log")

	var runningTests []load.Test

	port := 1454

	gin.DefaultWriter = io.MultiWriter(serverLogFile, os.Stdout)
	gin.DefaultErrorWriter = io.MultiWriter(serverLogFile, os.Stdout)

	registerInConsul(port)

	router := gin.Default()

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "Consul check")
	})
	router.POST("/run", func(c *gin.Context) {
		var test load.Test
		err := c.BindJSON(&test)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
		}
		err = runTest(&test)

		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			//processMiddlewareError(c, err)
		} else {
			c.String(http.StatusOK, "Тест запущен")
			runningTests = append(runningTests, test)
		}
	})

	/*router.POST("/stop/:name", func(c *gin.Context) {
		name := c.Param("name")
	}*/

	err := router.Run(fmt.Sprintf(":%d", port))
	log.Fatalf("ListenAndServe(): %v", err)
}

func runTest(test *load.Test) error {
	load.PrepareTest(test)

	go func() {
		test.Run()
	}()

	return nil
}

func registerInConsul(port int) {
	config := consulapi.DefaultConfig()
	config.Address = os.Getenv("consul_server_address")
	consul, err := consulapi.NewClient(config)

	if err != nil {
		log.Fatal(err.Error())
	}

	serviceID := "helloworld-server"
	address := os.Getenv("hostname")

	registration := &consulapi.AgentServiceRegistration{
		ID:      serviceID,
		Name:    "generator",
		Port:    port,
		Address: address,
		Check: &consulapi.AgentServiceCheck{
			HTTP:     fmt.Sprintf("http://%s:%d/health", address, port),
			Interval: "15s",
			Timeout:  "20s",
		},
		Tags: []string{"prometheus_monitoring_endpoint=/metrics"},
	}

	regiErr := consul.Agent().ServiceRegister(registration)

	if regiErr != nil {
		log.Fatalf("Failed to register service: %s:%v %s", address, port, regiErr.Error())
	} else {
		log.Printf("Successfully register service: %s:%v", address, port)
	}
}
