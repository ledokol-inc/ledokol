package load

import (
	"encoding/json"
	"fmt"
	"ledokol/kafkah"
	"math/rand"
	"os"
	"strconv"
	"time"
)

type Scenario struct {
	Name  string
	Steps []*Step
}

func InitScenariosFromConfig(fileName string) ([]Scenario, error) {
	result := make([]Scenario, 0)
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, err
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, err
	}
	for i, _ := range result {
		for j, _ := range result[i].Steps {
			result[i].Steps[j].FileContent, err = os.ReadFile("res/messages/" + result[i].Steps[j].FileName)
			if err != nil {
				return nil, err
			}
		}
	}
	return result, nil
}

func (scenario *Scenario) Process(producer *kafkah.ProducerWrapper, consumer *kafkah.ConsumerWrapper) {
	userId := rand.Int63n(99999999) + 100000000
	sessionId := rand.Int63n(99999999) + 100000000
	for _, step := range scenario.Steps {
		step.process(producer, userId, sessionId, consumer, scenario.Name)
	}
	fmt.Println("Итерация закончена")
}

type Step struct {
	Name        string
	FileName    string
	FileContent []byte
}

func (step *Step) process(producer *kafkah.ProducerWrapper, userId int64, sessionId int64, consumer *kafkah.ConsumerWrapper, scenarioName string) {
	messageId := userId + rand.Int63n(9999) + 10000
	changedMessage := kafkah.ReplaceAll(step.FileContent, messageId, sessionId, userId)
	producer.SendMessage([]byte(changedMessage))
	fmt.Printf("Сценарий %s, Шаг %s: сообщение отправлено\n", scenarioName, step.Name)
	currentTime := time.Now().UnixMilli()

	response := consumer.WaitForMessage(strconv.FormatInt(messageId, 10), 8000)

	fmt.Printf("Сценарий %s, Шаг %s: ", scenarioName, step.Name)
	if response == "" {
		fmt.Printf("Прошло 8 секунд, сообщения не было\n")
	} else {
		fmt.Printf("Сообщение получено за %d мс\n", time.Now().UnixMilli()-currentTime)
	}
}
