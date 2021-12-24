package load

import (
	"fmt"
	"ledokol/kafkah"
	"math/rand"
	"time"
)

type Test struct {
	pacing        int64
	consumer      *kafkah.ConsumerWrapper
	scenarios     []Scenario
	usersCount    int
	totalDuration int64
}

func NewTest() *Test {
	scenarios, err := InitScenariosFromConfig("res/scenarios.json")

	if err != nil {
		fmt.Printf("Файл конфигурации не найден")
		panic(err)
	}
	return &Test{
		pacing:        100e3,
		consumer:      kafkah.NewConsumer(),
		scenarios:     scenarios,
		usersCount:    10,
		totalDuration: 600,
	}
}

func (test *Test) Run() {
	go test.consumer.ProcessConsume()

	startTime := time.Now().Unix()
	for i := 0; i < test.usersCount; i++ {
		go func() {
			producer := kafkah.NewProducer()
			rand.Seed(time.Now().UnixNano())
			for time.Now().Unix()-startTime < test.totalDuration {
				timeBeforeTest := time.Now().UnixMilli()
				scenarioNumber := rand.Intn(len(test.scenarios))
				test.scenarios[scenarioNumber].Process(producer, test.consumer)
				timeAfterTest := time.Now().UnixMilli()
				if timeAfterTest-timeBeforeTest < test.pacing {
					time.Sleep(time.Duration(test.pacing-timeAfterTest+timeBeforeTest) * time.Millisecond)
				}
			}
		}()
	}
	time.Sleep(time.Duration(test.totalDuration) * time.Second)
}
