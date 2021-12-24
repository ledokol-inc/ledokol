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
		fmt.Println("Файл конфигурации не найден")
		panic(err)
	}
	return &Test{
		pacing:        60e3,
		consumer:      kafkah.NewConsumer(),
		scenarios:     scenarios,
		usersCount:    2,
		totalDuration: 1200,
	}
}

func (test *Test) Run() {
	go test.consumer.ProcessConsume()
	go test.consumer.DeleteOldMessages(10)

	startTime := time.Now().Unix()
	//test.StartUsers(test.usersCount, startTime)
	go test.StartUsersContinually(600, 10, 1, startTime)
	time.Sleep(time.Duration(test.totalDuration) * time.Second)
	test.consumer.Close()
}

func (test *Test) StartUsersContinually(totalCount int, countByPeriod int, period int, testStartTime int64) {
	for i := 0; i < totalCount; i += countByPeriod {
		test.StartUsers(countByPeriod, testStartTime)
		time.Sleep(time.Duration(period) * time.Second)
	}
}

func (test *Test) StartUsers(count int, testStartTime int64) {
	for i := 0; i < count; i++ {
		go func() {
			producer := kafkah.NewProducer()
			rand.Seed(time.Now().UnixNano())
			for time.Now().Unix()-testStartTime < test.totalDuration {
				timeBeforeTest := time.Now().UnixMilli()
				scenarioNumber := rand.Intn(len(test.scenarios))
				test.scenarios[scenarioNumber].Process(producer, test.consumer)
				timeAfterTest := time.Now().UnixMilli()
				if timeAfterTest-timeBeforeTest < test.pacing {
					time.Sleep(time.Duration(test.pacing-timeAfterTest+timeBeforeTest) * time.Millisecond)
				}
			}
			producer.Close()
		}()
	}
}
