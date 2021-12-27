package load

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
	"ledokol/kafkah"
	"math/rand"
	"os"
	"strconv"
	"sync"
	"time"
)

var TimeFormat = "2006-01-02 15:04:05"
var TestHistoryFileName = "res/tests/test_history.csv"

type Test struct {
	Name            string
	Pacing          float64
	PacingDelta     float64
	ScenariosName   string
	consumer        *kafkah.ConsumerWrapper
	scenarios       []Scenario
	Steps           []TestStep
	TotalDuration   float64
	stopUserChannel chan bool
}

func InitTestFromFile(fileName string) (*Test, error) {

	result := new(Test)
	data, err := os.ReadFile(fileName)
	if err != nil {
		return nil, errors.New("Файл с описанием теста не найден")
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, errors.New("Некорректное описание теста")
	}

	result.scenarios, err = InitScenariosFromFile(
		fmt.Sprintf("res/scenarios/%s/%s.json", result.ScenariosName, result.ScenariosName),
		fmt.Sprintf("res/scenarios/%s/messages/", result.ScenariosName))
	if err != nil {
		return nil, err
	}

	sumTimeBefore := 0.0
	for i := 0; i < len(result.Steps); i++ {
		if sumTimeBefore+result.Steps[i].Period > result.TotalDuration {
			result.Steps[i].Period = result.TotalDuration - sumTimeBefore
			result.Steps = result.Steps[:i+1]
			break
		}
		sumTimeBefore += result.Steps[i].Period
	}

	result.stopUserChannel = make(chan bool)
	result.consumer = kafkah.NewConsumer()
	return result, nil
}

func (test *Test) Run(id int) {
	go test.consumer.ProcessConsume()
	go test.consumer.DeleteOldMessages(10)

	usersCountMetric := promauto.NewGauge(prometheus.GaugeOpts{Name: "runner_users_running", Help: "Текущее количество работающих пользователей"})
	usersCountMetric.Set(0)

	startTime := time.Now().Unix()

	for _, step := range test.Steps {
		if step.Action == "start" {
			test.StartUsersContinually(step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000), usersCountMetric)
		} else if step.Action == "duration" {
			time.Sleep(time.Duration(step.Period*1000) * time.Millisecond)
		} else if step.Action == "stop" {
			test.StopUsersContinually(step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000))
		}
	}
	test.consumer.Close()
	historyFile, err := os.Open(TestHistoryFileName)
	defer historyFile.Close()
	if err == nil {
		test.writeTestInfo(historyFile, id, startTime, time.Now().Unix())
	}
}

type TestStep struct {
	Action             string
	TotalUsersCount    int
	CountUsersByPeriod int
	Period             float64
}

func (test *Test) StartUsersContinually(totalCount int, countByPeriod int, periodInMillis int, usersCountMetric prometheus.Gauge) {
	for i := 0; i < totalCount; i += countByPeriod {
		test.StartUsers(countByPeriod, usersCountMetric)
		time.Sleep(time.Duration(periodInMillis) * time.Millisecond)
	}
}

func (test *Test) StopUsersContinually(totalCount int, countByPeriod int, periodInMillis int) {
	stopUsersDone := &sync.WaitGroup{}
	for i := 0; i < totalCount; i += countByPeriod {
		stopUsersDone.Add(countByPeriod)
		go func() {
			for j := 0; j < countByPeriod; j++ {
				test.stopUserChannel <- true
				stopUsersDone.Done()
			}
		}()

		time.Sleep(time.Duration(periodInMillis) * time.Millisecond)
	}
	stopUsersDone.Wait()
}

func (test *Test) StartUsers(count int, usersCountMetric prometheus.Gauge) {
	for i := 0; i < count; i++ {
		go func() {
			producer := kafkah.NewProducer()
			rand.Seed(time.Now().UnixNano())
			usersCountMetric.Inc()
			for {
				timeBeforeTest := time.Now().UnixMilli()
				scenarioNumber := rand.Intn(len(test.scenarios))
				test.scenarios[scenarioNumber].Process(producer, test.consumer)
				currentPacing := ((rand.Float64()*2-1)*test.PacingDelta + 1) * test.Pacing
				timeToSleep := int64(currentPacing*1000) - time.Now().UnixMilli() + timeBeforeTest
				if timeToSleep < 1 {
					timeToSleep = 1
				}
				select {
				case _ = <-test.stopUserChannel:
					usersCountMetric.Dec()
					producer.Close()
					return
				case <-time.After(time.Duration(timeToSleep) * time.Millisecond):
					continue
				}
			}
		}()
	}
}

func (test *Test) writeTestInfo(historyFile *os.File, testId int, startTime int64, endTime int64) {
	writer := csv.NewWriter(historyFile)
	writer.Write([]string{strconv.Itoa(testId), time.Unix(startTime, 0).Format(TimeFormat), time.Unix(endTime, 0).Format(TimeFormat)})
}
