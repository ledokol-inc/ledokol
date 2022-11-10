package load

import (
	"ledokol/kafkah"
	"math/rand"
	"time"
)

type KafkaScenario struct {
	SimpleScenario
	consumer *kafkah.ConsumerWrapper
}

func (scenario *KafkaScenario) PrepareScenario(totalDuration float64) {
	scenario.SimpleScenario.PrepareScenario(totalDuration)
	scenario.consumer = kafkah.NewConsumer()
}

func (scenario *KafkaScenario) Run(testName string) int64 {
	if scenario.consumer != nil {
		go scenario.consumer.ProcessConsume()
		go scenario.consumer.DeleteOldMessages(10)
	}

	startTime := time.Now().Unix()

	for _, step := range scenario.Steps {
		if step.Action == "start" {
			StartUsersContinually(scenario, step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000), testName)
		} else if step.Action == "duration" {
			time.Sleep(time.Duration(step.Period*1000) * time.Millisecond)
		} else if step.Action == "stop" {
			scenario.StopUsersContinually(step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000))
		}
	}

	if scenario.consumer != nil {
		scenario.consumer.Close()
	}

	return startTime
}

func (scenario *KafkaScenario) StartUser(testName string) {
	producer := kafkah.NewProducer()
	rand.Seed(time.Now().UnixNano())
	usersCountMetric.WithLabelValues(testName, scenario.GetName()).Inc()
	for {
		timeBeforeTest := time.Now().UnixMilli()
		result, startIterationTime := scenario.Script.Process(producer, scenario.consumer, testName)
		if result {
			successScenarioCountMetric.WithLabelValues(testName, scenario.Name).Observe(float64(time.Now().UnixMilli()-startIterationTime) / 1000.0)
		} else {
			failedScenarioCountMetric.WithLabelValues(testName, scenario.Name).Inc()
		}
		currentPacing := ((rand.Float64()*2-1)*scenario.PacingDelta + 1) * scenario.Pacing
		timeToSleep := int64(currentPacing*1000) - time.Now().UnixMilli() + timeBeforeTest
		if timeToSleep < 1 {
			timeToSleep = 1
		}
		select {
		case _ = <-scenario.stopUserChannel:
			usersCountMetric.WithLabelValues(testName, scenario.GetName()).Dec()
			producer.Close()
			return
		case <-time.After(time.Duration(timeToSleep) * time.Millisecond):
			continue
		}
	}
}
