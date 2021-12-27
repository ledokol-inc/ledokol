package load

import (
	"encoding/json"
	"errors"
	"fmt"
	"ledokol/kafkah"
	"math/rand"
	"os"
	"strconv"
)

type Scenario struct {
	Name  string
	Steps []*Step
}

func InitScenariosFromFile(fileName string, messageFolder string) ([]Scenario, error) {
	result := make([]Scenario, 0)
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errors.New("Файл со сценариями для теста не найден")
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, errors.New("Файл сценариев для теста имеет неверный формат")
	}
	for i := range result {
		for j := range result[i].Steps {
			result[i].Steps[j].FileContent, err = os.ReadFile(messageFolder + result[i].Steps[j].FileName)
			if err != nil {
				return nil, errors.New(fmt.Sprintf("Файл с сообщением для сценария \"%s\" шага \"%s\" не найден", result[i].Name, result[i].Steps[j].Name))
			}
		}
	}
	return result, nil
}

func (scenario *Scenario) Process(producer *kafkah.ProducerWrapper, consumer *kafkah.ConsumerWrapper) {
	userId := rand.Int63n(99999999) + 100000000
	sessionId := rand.Int63n(99999999) + 100000000
	for _, step := range scenario.Steps {
		step.process(producer, userId, sessionId, consumer)
	}
	//fmt.Println("Итерация закончена")
}

type Step struct {
	Name        string
	FileName    string
	FileContent []byte
}

func (step *Step) process(producer *kafkah.ProducerWrapper, userId int64, sessionId int64, consumer *kafkah.ConsumerWrapper) {
	messageId := userId + rand.Int63n(9999) + 10000
	changedMessage := kafkah.ReplaceAll(step.FileContent, messageId, sessionId, userId)
	producer.SendMessage([]byte(changedMessage))
	//fmt.Printf("Сценарий %s, Шаг %s: сообщение отправлено, message id = %d\n", scenarioName, step.Name, messageId)
	//currentTime := time.Now().UnixMilli()

	response := consumer.WaitForMessage(strconv.FormatInt(messageId, 10), 8000)

	//fmt.Printf("Сценарий %s, Шаг %s: ", scenarioName, step.Name)
	if response == "" {
		//fmt.Printf("Прошло 8 секунд, сообщения не было\n")
	} else {
		//fmt.Printf("Сообщение получено за %d мс\n", time.Now().UnixMilli()-currentTime)
	}
}
