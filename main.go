package main

import (
	"bytes"
	"fmt"
	"github.com/gin-gonic/gin"
	consulapi "github.com/hashicorp/consul/api"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
	"ledokol/discovery"
	"ledokol/load"
	"ledokol/logger"
	"net/http"
	"os"
	"os/signal"
	"syscall"
)

const port = 1455
const logFile = "server.log"

func main() {

	runningTests := make(map[string]*load.Test)

	fileLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    5,
		MaxBackups: 10,
		MaxAge:     14,
		Compress:   true,
	}

	log.Logger = log.Output(zerolog.MultiLevelWriter(os.Stderr, fileLogger))

	router := gin.New()
	router.Use(logger.Logger())

	consulAgent, serviceId := discovery.RegisterInConsul(port)

	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interruptChan
		consulAgent.ServiceDeregister(serviceId)
		log.Info().Msg("Service deregistered")
		os.Exit(1)
	}()

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(c *gin.Context) {
		c.String(http.StatusOK, "Consul check")
	})
	router.POST("/run", func(c *gin.Context) {
		/*var test load.Test
		err := c.BindJSON(&test)*/
		var test load.Test
		if err := c.BindJSON(&test); err != nil {
			c.String(http.StatusBadRequest, err.Error())
			return
		}

		if err := runTest(&test, runningTests, consulAgent); err != nil {
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
	log.Fatal().Err(err).Msg("ListenAndServe() error")
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

func sendEndTestRequestToMain(agent *consulapi.Agent, testId string) {
	service, _, err := agent.Service("ledokol-main", &consulapi.QueryOptions{})
	if err != nil {
		log.Error().Err(err).Msg("Failed to find ledokol-main")
		return
	}

	address := service.Address
	port := service.Port
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
