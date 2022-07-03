package load

import (
	"ledokol/kafkah"
	"math/rand"
	"strconv"
	"strings"
	"sync"
	"time"
)

var TimeFormat = "2006-01-02 15:04:05"

type Test struct {
	Name            string
	Pacing          float64
	PacingDelta     float64
	ScenariosName   string
	consumer        *kafkah.ConsumerWrapper
	Scenarios       []Scenario
	Steps           []TestStep
	TotalDuration   float64
	stopUserChannel chan bool
	id              string
}

func PrepareTest(test *Test) {

	sumTimeBefore := 0.0
	for i := 0; i < len(test.Steps); i++ {
		if sumTimeBefore+test.Steps[i].Period > test.TotalDuration {
			test.Steps[i].Period = test.TotalDuration - sumTimeBefore
			test.Steps = test.Steps[:i+1]
			break
		}
		sumTimeBefore += test.Steps[i].Period
	}

	test.stopUserChannel = make(chan bool)
	if !strings.Contains(test.Name, "tstub") {
		test.consumer = kafkah.NewConsumer()
	}
}

func (test *Test) Run(id int) int64 {
	test.id = strconv.Itoa(id)
	usersCountMetric.WithLabelValues(test.id).Set(0)
	if test.consumer != nil {
		go test.consumer.ProcessConsume()
		go test.consumer.DeleteOldMessages(10)
	}

	startTime := time.Now().Unix()

	for _, step := range test.Steps {
		if step.Action == "start" {
			test.StartUsersContinually(step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000))
		} else if step.Action == "duration" {
			time.Sleep(time.Duration(step.Period*1000) * time.Millisecond)
		} else if step.Action == "stop" {
			test.StopUsersContinually(step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000))
		}
	}

	if test.consumer != nil {
		test.consumer.Close()
	}

	return startTime
}

type TestStep struct {
	Action             string
	TotalUsersCount    int
	CountUsersByPeriod int
	Period             float64
}

func (test *Test) StartUsersContinually(totalCount int, countByPeriod int, periodInMillis int) {
	for i := 0; i < totalCount; i += countByPeriod {
		test.StartUsers(countByPeriod)
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

func (test *Test) StartUsers(count int) {
	for i := 0; i < count; i++ {
		go func() {
			var producer *kafkah.ProducerWrapper
			if test.consumer != nil {
				producer = kafkah.NewProducer()
			}
			rand.Seed(time.Now().UnixNano())
			usersCountMetric.WithLabelValues(test.id).Inc()
			for {
				timeBeforeTest := time.Now().UnixMilli()
				scenarioNumber := rand.Intn(len(test.Scenarios))
				if test.consumer != nil {
					test.Scenarios[scenarioNumber].Process(producer, test.consumer, test.id)
				} else {
					test.Scenarios[scenarioNumber].ProcessHttp(test.id)
				}
				currentPacing := ((rand.Float64()*2-1)*test.PacingDelta + 1) * test.Pacing
				timeToSleep := int64(currentPacing*1000) - time.Now().UnixMilli() + timeBeforeTest
				if timeToSleep < 1 {
					timeToSleep = 1
				}
				select {
				case _ = <-test.stopUserChannel:
					usersCountMetric.WithLabelValues(test.id).Dec()
					if test.consumer != nil {
						producer.Close()
					}
					return
				case <-time.After(time.Duration(timeToSleep) * time.Millisecond):
					continue
				}
			}
		}()
	}
}
