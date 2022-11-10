package load

import (
	"bytes"
	"ledokol/kafkah"
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
	for i := 0; i < len(script.Steps); i++ {
		startTime := time.Now().UnixMilli()
		resp, err := http.Post(script.Steps[i].Url, "application/json", bytes.NewBufferString(script.Steps[i].Message))
		//fmt.Printf("Сценарий %s, Шаг %s: сообщение отправлено, message id = %d\n", scenarioName, step.Name, messageId)
		//currentTime := time.Now().UnixMilli()

		//fmt.Printf("Сценарий %s, Шаг %s: ", scenarioName, step.Name)
		if err != nil {
			failedTransactionCountMetric.WithLabelValues(testName, script.Name, script.Steps[i].Name, "true").Inc()
			success = false
			println(err.Error())
			break
			//fmt.Printf("Прошло 8 секунд, сообщения не было\n")
		} else {
			successTransactionCountMetric.WithLabelValues(testName, script.Name, script.Steps[i].Name).
				Observe(float64(time.Now().UnixMilli()-startTime) / 1000.0)
			resp.Body.Close()
			//fmt.Printf("Сообщение получено за %d мс\n", time.Now().UnixMilli()-currentTime)
		}
	}

	return success, startIterationTime
}

type Step struct {
	Name     string
	FileName string
	Message  string `json:"body"`
	Url      string
}
