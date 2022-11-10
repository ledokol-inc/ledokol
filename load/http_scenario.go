package load

import (
	"math/rand"
	"sync"
	"time"
)

func (scenario *SimpleScenario) PrepareScenario(totalDuration float64) {

	sumTimeBefore := 0.0
	for i := 0; i < len(scenario.Steps); i++ {
		if sumTimeBefore+scenario.Steps[i].Period > totalDuration {
			scenario.Steps[i].Period = totalDuration - sumTimeBefore
			scenario.Steps = scenario.Steps[:i+1]
			break
		}
		sumTimeBefore += scenario.Steps[i].Period
	}

	scenario.stopUserChannel = make(chan bool)
}

func (scenario *SimpleScenario) Run(testName string) int64 {
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

	return startTime
}

func StartUsers(scenario Scenario, count int, testName string) {
	for i := 0; i < count; i++ {
		go func() {
			scenario.StartUser(testName)
		}()
	}
}

func (scenario *SimpleScenario) StartUser(testName string) {
	rand.Seed(time.Now().UnixNano())
	usersCountMetric.WithLabelValues(testName, scenario.GetName()).Inc()
	for {
		timeBeforeTest := time.Now().UnixMilli()
		result, startIterationTime := scenario.Script.ProcessHttp(testName)
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
			return
		case <-time.After(time.Duration(timeToSleep) * time.Millisecond):
			continue
		}
	}
}

func StartUsersContinually(scenario Scenario, totalCount int, countByPeriod int, periodInMillis int, testName string) {
	for i := 0; i < totalCount; i += countByPeriod {
		StartUsers(scenario, countByPeriod, testName)
		time.Sleep(time.Duration(periodInMillis) * time.Millisecond)
	}
}

func (scenario *SimpleScenario) StopUsersContinually(totalCount int, countByPeriod int, periodInMillis int) {
	stopUsersDone := &sync.WaitGroup{}
	for i := 0; i < totalCount; i += countByPeriod {
		stopUsersDone.Add(countByPeriod)
		go func() {
			for j := 0; j < countByPeriod; j++ {
				scenario.stopUserChannel <- true
				stopUsersDone.Done()
			}
		}()

		time.Sleep(time.Duration(periodInMillis) * time.Millisecond)
	}
	stopUsersDone.Wait()
}

func (scenario *SimpleScenario) GetName() string {
	return scenario.Name
}
