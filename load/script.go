package load

import (
	"bytes"
	"io"
	"ledokol/kafkah"
	"log"
	"math/rand"
	"net/http"
	"strconv"
	"strings"
	"time"
)

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

func (script *Script) Process(producer *kafkah.ProducerWrapper, consumer *kafkah.ConsumerWrapper, testName string) (bool, int64) {
	userId := rand.Int63n(99999999) + 100000000
	sessionId := rand.Int63n(99999999) + 100000000
	success := true
	startIterationTime := time.Now().UnixMilli()
	for i := 0; i < len(script.Steps); i++ {
		messageId := userId + rand.Int63n(9999) + 10000
		changedMessage := kafkah.ReplaceAll([]byte(script.Steps[i].Message), messageId, sessionId, userId)
		startTime := time.Now().UnixMilli()
		producer.SendMessage([]byte(changedMessage))
		//fmt.Printf("Сценарий %s, Шаг %s: сообщение отправлено, message id = %d\n", scenarioName, step.Name, messageId)
		//currentTime := time.Now().UnixMilli()

		response := consumer.WaitForMessage(strconv.FormatInt(messageId, 10), 8000)

		//fmt.Printf("Сценарий %s, Шаг %s: ", scenarioName, step.Name)
		if response == "" {
			failedTransactionCountMetric.WithLabelValues(testName, script.Name, script.Steps[i].Name, "true").Inc()
			success = false
			break
			//fmt.Printf("Прошло 8 секунд, сообщения не было\n")
		} else {
			containsFinished := strings.Contains(response, "\"finished\": true")
			isEnded := i == len(script.Steps)-1
			if !isEnded && containsFinished || isEnded && !containsFinished {
				failedTransactionCountMetric.WithLabelValues(testName, script.Name, script.Steps[i].Name, "false").Inc()
				success = false
				break
			} else {
				successTransactionCountMetric.WithLabelValues(testName, script.Name, script.Steps[i].Name).
					Observe(float64(time.Now().UnixMilli()-startTime) / 1000.0)
			}
			//fmt.Printf("Сообщение получено за %d мс\n", time.Now().UnixMilli()-currentTime)
		}
	}
	return success, startIterationTime
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
			log.Fatalf("%s", err.Error())
		}

		for key, value := range step.Headers {
			req.Header.Set(key, value)
		}

		resp, err := step.httpClient.Do(req)

		//fmt.Printf("Сценарий %s, Шаг %s: сообщение отправлено, message id = %d\n", scenarioName, step.Name, messageId)

		if err != nil || resp.StatusCode >= 200 && resp.StatusCode < 300 {
			failedTransactionCountMetric.WithLabelValues(testName, script.Name, step.Name, "true").Inc()
			success = false
			if err != nil {
				log.Printf(err.Error())
			} else {
				body, err := getResponseBody(resp)
				if err == nil {
					log.Printf("Получен ответ: %s", body)
				} else {
					log.Printf("При попытке чтения ответа произошла ошибка: %s", err.Error())
				}
			}
			break
			//fmt.Printf("Прошло 8 секунд, сообщения не было\n")
		} else {
			successTransactionCountMetric.WithLabelValues(testName, script.Name, step.Name).
				Observe(float64(time.Now().UnixMilli()-startTime) / 1000.0)
			resp.Body.Close()
			//fmt.Printf("Сообщение получено за %d мс\n", time.Now().UnixMilli()-currentTime)
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
