package load

import (
	"bytes"
	"crypto/rand"
	"encoding/base64"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"net/http"
	"strconv"
	"time"
)

const requestIdLength = 15

type Script struct {
	Name  string
	Steps []*Step
}

type Step struct {
	Name       string
	Message    string `json:"body"`
	Url        string
	Method     string
	Headers    map[string]string
	httpClient *http.Client
	Timeout    int64
}

func (script *Script) ProcessHttp(testName string) (bool, int64) {
	success := true
	startIterationTime := time.Now().UnixMilli()
	for _, step := range script.Steps {
		startTime := time.Now().UnixMilli()

		var req *http.Request
		var err error

		if step.Message == "" {
			req, err = http.NewRequest(step.Method, step.Url, nil)
		} else {
			req, err = http.NewRequest(step.Method, step.Url, bytes.NewBufferString(step.Message))
		}

		if err != nil {
			log.Error().Err(err).Msgf("")
			break
		}

		for key, value := range step.Headers {
			req.Header.Set(key, value)
		}

		var requestId string

		if requestId, err = randomId(requestIdLength); err != nil {
			log.Error().Err(err).Msgf("Не удалось инициализировать request ID")
			break
		}

		resp, err := step.httpClient.Do(req)

		log.Info().Str("test", testName).Str("script", script.Name).
			Str("step", step.Name).Str("body", step.Message).
			Str("requestId", requestId).Msg("Отправка запроса")

		if err != nil || resp.StatusCode >= 300 {
			failedTransactionCountMetric.WithLabelValues(testName, script.Name, step.Name, "true").Inc()
			success = false
			if err != nil {
				log.Error().Err(err).Str("test", testName).
					Str("script", script.Name).Str("step", step.Name).
					Str("requestId", requestId).Msg("Ошибка отправки запроса")
			} else {
				body, err := getResponseBody(resp)
				if err != nil {
					logReadResponseError(err, testName, script.Name, step.Name, resp.StatusCode, requestId)
				} else {
					logReadResponse(err, testName, script.Name, step.Name, resp.StatusCode, requestId, body, true)
				}
			}
			break
		} else {
			successTransactionCountMetric.WithLabelValues(testName, script.Name, step.Name).
				Observe(float64(time.Now().UnixMilli()-startTime) / 1000.0)

			body, err := getResponseBody(resp)
			if err != nil {
				logReadResponseError(err, testName, script.Name, step.Name, resp.StatusCode, requestId)
			} else {
				logReadResponse(err, testName, script.Name, step.Name, resp.StatusCode, requestId, body, false)
			}
		}
	}

	return success, startIterationTime
}

func getResponseBody(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	resp.Body.Close()
	return string(body), nil
}

func randomId(length int) (string, error) {
	b := make([]byte, length)
	if _, err := rand.Read(b); err != nil {
		return "", err
	}

	return base64.RawURLEncoding.EncodeToString(b), nil
}

func logReadResponseError(err error, testName string, scriptName string, stepName string, status int, requestId string) {
	log.Error().Err(err).Str("test", testName).Str("script", scriptName).
		Str("step", stepName).Str("status", strconv.Itoa(status)).
		Str("requestId", requestId).Msg("При попытке чтения ответа произошла ошибка")
}

func logReadResponse(err error, testName string, scriptName string, stepName string, status int, requestId string, body string, isError bool) {
	var event *zerolog.Event
	if isError {
		event = log.Error()
	} else {
		event = log.Info()
	}
	event.Str("test", testName).Str("script", scriptName).
		Str("step", stepName).Str("body", body).
		Str("status", strconv.Itoa(status)).Str("requestId", requestId).Msg("Получен ответ")
}
