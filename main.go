package main

import (
	"fmt"
	"math/rand"
	"net/http"
	"os"
	"os/signal"
	"reflect"
	"regexp"
	"regexp/syntax"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/mitchellh/mapstructure"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"github.com/spf13/viper"
	"gopkg.in/natefinch/lumberjack.v2"

	"github.com/ledokol-inc/ledokol/discovery"
	"github.com/ledokol-inc/ledokol/load"
	"github.com/ledokol-inc/ledokol/logger"
)

const portDefault = 1455
const defaultLogLevel = "info"

func main() {

	rand.Seed(time.Now().UnixNano())
	runningTests := make(map[string]*load.Test)

	viper.SetConfigFile("config.yaml")
	err := viper.ReadInConfig()
	if err != nil {
		panic(fmt.Errorf("не удалось инициализировать конфигурацию\n %w", err))
	}

	viper.SetDefault("logging.level", defaultLogLevel)
	_ = viper.BindEnv("logging.level", "log-level")

	loggingLevel := viper.GetString("logging.level")
	fileLogger := &lumberjack.Logger{
		Filename:   viper.GetString("logging.file"),
		MaxSize:    viper.GetInt("logging.max-file-size"),
		MaxBackups: viper.GetInt("logging.max-backups"),
		MaxAge:     viper.GetInt("logging.max-age"),
		Compress:   viper.GetBool("logging.compress-rotated-log"),
	}
	level, err := zerolog.ParseLevel(loggingLevel)
	if err != nil {
		level = zerolog.InfoLevel
	}
	zerolog.SetGlobalLevel(level)
	zerolog.TimeFieldFormat = viper.GetString("logging.time-format")

	standardOutput := viper.GetString("logging.standard-output")
	if standardOutput == "stderr" {
		log.Logger = log.Output(zerolog.MultiLevelWriter(os.Stderr, fileLogger))
	} else if standardOutput == "stdout" {
		log.Logger = log.Output(zerolog.MultiLevelWriter(os.Stdout, fileLogger))
	} else {
		log.Logger = log.Output(zerolog.MultiLevelWriter(fileLogger))
	}

	router := gin.New()
	router.Use(logger.Logger())

	viper.SetDefault("server.http-port", portDefault)
	_ = viper.BindEnv("server.http-port", "http_port")

	port := viper.GetInt("server.http-port")

	service := discovery.RegisterInConsul(port)

	interruptChan := make(chan os.Signal, 1)
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
