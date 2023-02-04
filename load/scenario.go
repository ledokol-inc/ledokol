package load

import (
	"math/rand"
	"sync"
	"sync/atomic"
	"time"
)

type Scenario struct {
	Name                string
	stopUserChannel     chan bool
	Steps               []ScenarioStep
	Pacing              float64
	PacingDelta         float64
	Script              *Script
	stopScenarioChannel chan struct{}
	stopped             atomic.Bool
	runningUserWait     *sync.WaitGroup
}

type ScenarioStep struct {
	Action             string
	TotalUsersCount    int
	CountUsersByPeriod int
	Period             float64
}

func (scenario *Scenario) PrepareScenario(totalDuration float64) {
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
	scenario.stopScenarioChannel = make(chan struct{})
	scenario.runningUserWait = &sync.WaitGroup{}
}

func (scenario *Scenario) StartUsersContinually(totalCount int, countByPeriod int, periodInMillis int, testName string) {
	for i := 0; i < totalCount; i += countByPeriod {
		scenario.StartUsers(countByPeriod, testName)
		time.Sleep(time.Duration(periodInMillis) * time.Millisecond)
	}
}

func (scenario *Scenario) StartUsers(count int, testName string) {
	for i := 0; i < count; i++ {
		go func() {
			if !scenario.stopped.Load() {
				scenario.runningUserWait.Add(1)
				scenario.StartUser(testName)
				scenario.runningUserWait.Done()
			}
		}()
	}
}

func (scenario *Scenario) Run(testName string) int64 {
	startTime := time.Now().Unix()

	for _, step := range scenario.Steps {

		if scenario.stopped.Load() {
			break
		}

		if step.Action == "start" {
			scenario.StartUsersContinually(step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000), testName)
		} else if step.Action == "duration" {
			select {
			case _ = <-scenario.stopScenarioChannel:
				continue
			case <-time.After(time.Duration(step.Period*1000) * time.Millisecond):
				continue
			}

		} else if step.Action == "stop" {
			scenario.StopUsersContinually(step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000))
		}
	}

	close(scenario.stopUserChannel)

	if !scenario.stopped.CompareAndSwap(false, true) {
		<-scenario.stopScenarioChannel
	}

	scenario.runningUserWait.Wait()

	return startTime
}

func (scenario *Scenario) StopUsersContinually(totalCount int, countByPeriod int, periodInMillis int) {
	stopUsersDone := &sync.WaitGroup{}
	for i := 0; i < totalCount; i += countByPeriod {
		if scenario.stopped.Load() {
			break
		}

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

func (scenario *Scenario) StartUser(testName string) {
	userRand := initRand()
	usersCountMetric.WithLabelValues(testName, scenario.Name).Inc()
	scriptVariables := scenario.Script.PrepareScript(userRand)
	for {
		timeBeforeTest := time.Now().UnixMilli()
		result, startIterationTime := scenario.Script.ProcessHttp(testName, userRand, scriptVariables)
		if result {
			successScenarioCountMetric.WithLabelValues(testName, scenario.Name).Observe(float64(time.Now().UnixMilli()-startIterationTime) / 1000.0)
		} else {
			failedScenarioCountMetric.WithLabelValues(testName, scenario.Name).Inc()
		}
		currentPacing := ((userRand.Float64()*2-1)*scenario.PacingDelta + 1) * scenario.Pacing
		timeToSleep := int64(currentPacing*1000) - time.Now().UnixMilli() + timeBeforeTest
		if timeToSleep < 1 {
			timeToSleep = 1
		}
		select {
		case _ = <-scenario.stopUserChannel:
			usersCountMetric.WithLabelValues(testName, scenario.Name).Dec()
			return
		case <-time.After(time.Duration(timeToSleep) * time.Millisecond):
			continue
		}
	}
}

func (scenario *Scenario) Stop() {
	if scenario.stopped.CompareAndSwap(false, true) {
		scenario.stopScenarioChannel <- struct{}{}
		close(scenario.stopScenarioChannel)
	}
}

func initRand() *rand.Rand {
	return rand.New(rand.NewSource(rand.Int63()))
}
