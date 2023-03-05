package main

import (
	"fmt"
	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"gopkg.in/natefinch/lumberjack.v2"
	"ledokol/discovery"
	"ledokol/load"
	"ledokol/logger"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"regexp/syntax"
	"strconv"
	"syscall"
	"time"
)

const portDefault = 1455
const logFile = "./logs/server.log"

func main() {

	rand.Seed(time.Now().UnixNano())
	runningTests := make(map[string]*load.Test)

	fileLogger := &lumberjack.Logger{
		Filename:   logFile,
		MaxSize:    5,
		MaxBackups: 10,
		MaxAge:     14,
		Compress:   true,
	}

	zerolog.TimeFieldFormat = "2006-01-02 15:04:05.000"
	log.Logger = log.Output(zerolog.MultiLevelWriter(os.Stderr, fileLogger))

	router := gin.New()
	router.Use(logger.Logger())

	port, err := strconv.Atoi(os.Getenv("http_port"))

	if err != nil {
		log.Info().Msgf("Переменная среды http_port не задана или задана неверно, используется порт по умолчанию: %d", portDefault)
		port = portDefault
	}

	service := discovery.RegisterInConsul(port)

	interruptChan := make(chan os.Signal)
	signal.Notify(interruptChan, os.Interrupt, syscall.SIGTERM)
	go func() {
		<-interruptChan
		if service != nil {
			service.DeregisterInConsul()
		}
		os.Exit(1)
	}()

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/health", func(c *gin.Context) {
		c.JSON(http.StatusOK, gin.H{"message": "Consul check"})
	})
	router.POST("/run", func(c *gin.Context) {
		var testData map[string]interface{}
		if err := c.ShouldBindJSON(&testData); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}
		var options load.TestOptions
		var test load.Test
		decoder, err := mapstructure.NewDecoder(&mapstructure.DecoderConfig{
			Result: &test,
			DecodeHook: mapstructure.ComposeDecodeHookFunc(
				unmarshalSyntaxRegexp,
				unmarshalStandardRegexp,
			),
		})
		if err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := decoder.Decode(testData["test"]); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		if err := mapstructure.Decode(testData["options"], &options); err != nil {
			c.JSON(http.StatusBadRequest, gin.H{"error": err.Error()})
			return
		}

		test.SetOptions(&options)

		if err := runTest(&test, runningTests, service); err != nil {
			c.JSON(http.StatusInternalServerError, gin.H{"error": err.Error()})
		} else {
			c.JSON(http.StatusOK, gin.H{"message": "Тест запущен"})
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

	err = router.Run(fmt.Sprintf(":%d", port))
	log.Fatal().Err(err).Msg("ListenAndServe() error")
}

func runTest(test *load.Test, runningTests map[string]*load.Test, service *discovery.Service) error {
	test.PrepareTest()

	go func(runningTests map[string]*load.Test, test *load.Test) {
		test.Run()
		if _, contains := runningTests[test.Id]; contains {
			delete(runningTests, test.Id)
			if service != nil {
				service.SendEndTestRequestToMain(test.Id)
			}
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

func unmarshalSyntaxRegexp(_ reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
	if to != reflect.TypeOf(&syntax.Regexp{}) {
		return data, nil
	}

	regexString, ok := data.(string)
	if !ok {
		return nil, fmt.Errorf("can't read regex string from %v", data)
	}

	return syntax.Parse(regexString, syntax.Perl)
}

func unmarshalStandardRegexp(_ reflect.Type, to reflect.Type, data interface{}) (interface{}, error) {
	if to != reflect.TypeOf(&regexp.Regexp{}) {
		return data, nil
	}

	regexString, ok := data.(string)
	if !ok {
		return nil, fmt.Errorf("can't read regex string from %v", data)
	}

	return regexp.Compile(regexString)
}
