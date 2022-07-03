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

type Scenario struct {
	Name  string
	Steps []*Step
}

func (scenario *Scenario) Process(producer *kafkah.ProducerWrapper, consumer *kafkah.ConsumerWrapper, testId string) {
	userId := rand.Int63n(99999999) + 100000000
	sessionId := rand.Int63n(99999999) + 100000000
	success := true
	startScenarioTime := time.Now().UnixMilli()
	for i := 0; i < len(scenario.Steps); i++ {
		messageId := userId + rand.Int63n(9999) + 10000
		changedMessage := kafkah.ReplaceAll([]byte(scenario.Steps[i].Message), messageId, sessionId, userId)
		startTime := time.Now().UnixMilli()
		producer.SendMessage([]byte(changedMessage))
		//fmt.Printf("Сценарий %s, Шаг %s: сообщение отправлено, message id = %d\n", scenarioName, step.Name, messageId)
		//currentTime := time.Now().UnixMilli()

		response := consumer.WaitForMessage(strconv.FormatInt(messageId, 10), 8000)

		//fmt.Printf("Сценарий %s, Шаг %s: ", scenarioName, step.Name)
		if response == "" {
			failedTransactionCountMetric.WithLabelValues(testId, scenario.Name, scenario.Steps[i].Name, "true").Inc()
			success = false
			break
			//fmt.Printf("Прошло 8 секунд, сообщения не было\n")
		} else {
			containsFinished := strings.Contains(response, "\"finished\": true")
			isEnded := i == len(scenario.Steps)-1
			if !isEnded && containsFinished || isEnded && !containsFinished {
				failedTransactionCountMetric.WithLabelValues(testId, scenario.Name, scenario.Steps[i].Name, "false").Inc()
				success = false
				break
			} else {
				successTransactionCountMetric.WithLabelValues(testId, scenario.Name, scenario.Steps[i].Name).
					Observe(float64(time.Now().UnixMilli()-startTime) / 1000.0)
			}
			//fmt.Printf("Сообщение получено за %d мс\n", time.Now().UnixMilli()-currentTime)
		}
	}
	if success {
		successScenarioCountMetric.WithLabelValues(testId, scenario.Name).Observe(float64(time.Now().UnixMilli()-startScenarioTime) / 1000.0)
	} else {
		failedScenarioCountMetric.WithLabelValues(testId, scenario.Name).Inc()
	}
	//fmt.Println("Итерация закончена")
}

func (scenario *Scenario) ProcessHttp(testId string) {
	success := true
	startScenarioTime := time.Now().UnixMilli()
	for i := 0; i < len(scenario.Steps); i++ {
		startTime := time.Now().UnixMilli()
		resp, err := http.Post(scenario.Steps[i].Url, "application/json", bytes.NewBufferString(scenario.Steps[i].Message))
		//fmt.Printf("Сценарий %s, Шаг %s: сообщение отправлено, message id = %d\n", scenarioName, step.Name, messageId)
		//currentTime := time.Now().UnixMilli()

		//fmt.Printf("Сценарий %s, Шаг %s: ", scenarioName, step.Name)
		if err != nil {
			failedTransactionCountMetric.WithLabelValues(testId, scenario.Name, scenario.Steps[i].Name, "true").Inc()
			success = false
			println(err.Error())
			break
			//fmt.Printf("Прошло 8 секунд, сообщения не было\n")
		} else {
			successTransactionCountMetric.WithLabelValues(testId, scenario.Name, scenario.Steps[i].Name).
				Observe(float64(time.Now().UnixMilli()-startTime) / 1000.0)
			resp.Body.Close()
			//fmt.Printf("Сообщение получено за %d мс\n", time.Now().UnixMilli()-currentTime)
		}
	}
	if success {
		successScenarioCountMetric.WithLabelValues(testId, scenario.Name).Observe(float64(time.Now().UnixMilli()-startScenarioTime) / 1000.0)
	} else {
		failedScenarioCountMetric.WithLabelValues(testId, scenario.Name).Inc()
	}
	//fmt.Println("Итерация закончена")
}

type Step struct {
	Name     string
	FileName string
	Message  string `json:"body"`
	Url      string
}
