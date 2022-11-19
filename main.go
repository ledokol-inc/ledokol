package main

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"ledokol/load"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

func main() {

	serverLogFile, _ := os.Create("server.log")

	runningTests := make(map[string]*load.Test)

	port := 1455

	gin.DefaultWriter = io.MultiWriter(serverLogFile, os.Stdout)
	gin.DefaultErrorWriter = io.MultiWriter(serverLogFile, os.Stdout)

	consulAgent, serviceId := registerInConsul(port)

	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interruptChan
		consulAgent.ServiceDeregister(serviceId)
		log.Printf("Service deregistered")
		os.Exit(1)
	}()

	router := gin.Default()

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "Consul check")
	})
	router.POST("/run", func(c *gin.Context) {
		/*var test load.Test
		err := c.BindJSON(&test)*/
		var test load.Test
		err := c.BindJSON(&test)
		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
		}
		err = runTest(&test, runningTests, consulAgent)

		if err != nil {
			c.String(http.StatusInternalServerError, err.Error())
			//processMiddlewareError(c, err)
		} else {
			c.String(http.StatusOK, "Тест запущен")
			runningTests[test.Id] = &test
		}
	})

	router.POST("/:id/stop", func(c *gin.Context) {
		id := c.Param("id")
		if stopTest(id, runningTests) {
			c.String(http.StatusOK, "Остановка теста запущена")
		} else {
			c.String(http.StatusNotFound, "Тест с таким id не запущен")
		}
	})

	err := router.Run(fmt.Sprintf(":%d", port))
	log.Fatalf("ListenAndServe(): %v", err)
}

func runTest(test *load.Test, runningTests map[string]*load.Test, agent *consulapi.Agent) error {
	test.PrepareTest()

	go func(runningTests map[string]*load.Test, test *load.Test) {
		test.Run()
		if _, contains := runningTests[test.Id]; contains {
			delete(runningTests, test.Id)
			sendEndTestRequestToMain(agent, test.Id)
		}
	}(runningTests, test)

	return nil
}

func stopTest(testId string, runningTests map[string]*load.Test) bool {
	test, exist := runningTests[testId]
	if exist {
		delete(runningTests, testId)
		test.Stop()
		return true
	} else {
		return false
	}
}

func registerInConsul(port int) (*consulapi.Agent, string) {

	name := "generator"

	config := consulapi.DefaultConfig()
	config.Address = os.Getenv("consul_server_address")
	consul, err := consulapi.NewClient(config)

	if err != nil {
		log.Fatal(err.Error())
	}

	address := os.Getenv("HOSTNAME")
	serviceID := fmt.Sprintf("%s-%s-%d", name, address, port)

	registration := &consulapi.AgentServiceRegistration{
		ID:   serviceID,
		Name: name,
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
		log.Fatalf("Failed to register service: %s:%v %s", address, port, regiErr.Error())
	} else {
		log.Printf("Successfully register service: %s:%v\n", address, port)
	}
	return consul.Agent(), serviceID
}

func sendEndTestRequestToMain(agent *consulapi.Agent, testId string) {
	service, _, err := agent.Service("ledokol-main", &consulapi.QueryOptions{})
	if err != nil {
		log.Printf("Failed to find ledokol-main: %s\n", err.Error())
		return
	}

	address := service.Address
	port := service.Port
	url := fmt.Sprintf("http://%s:%d/api/testruns/%s/end", address, port, testId)
	response, err := http.Post(url, "application/json", &bytes.Buffer{})
	if err != nil {
		log.Printf("Failed to send end test request to main component: %s\n", err.Error())
		return
	}
	if response.StatusCode != 200 {
		log.Printf("Status %d when send end test request to main component\n", response.StatusCode)
		return
	}

	log.Printf("Successfully sent end test request to main component\n")
}
