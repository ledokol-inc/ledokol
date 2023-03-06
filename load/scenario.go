package load

import (
	"sync"
	"sync/atomic"
	"time"
)

type StepAction string

const (
	StartAction    StepAction = "start"
	DurationAction            = "duration"
	StopAction                = "stop"
)

type Scenario struct {
	Name                string
	stopUserChannel     chan struct{}
	Steps               []ScenarioStep
	Pacing              float64
	PacingDelta         float64
	Script              *Script
	stopScenarioChannel chan struct{}
	stopped             atomic.Bool
	runningUserWait     *sync.WaitGroup
}

type ScenarioStep struct {
	Action             StepAction
	TotalUsersCount    int
	CountUsersByPeriod int
	Period             float64
}

func (scenario *Scenario) PrepareScenario(totalDuration float64) {
	if totalDuration != 0 {
		sumTimeBefore := 0.0
		for i := 0; i < len(scenario.Steps); i++ {
			if scenario.Steps[i].Action == DurationAction && sumTimeBefore+scenario.Steps[i].Period > totalDuration {
				scenario.Steps[i].Period = totalDuration - sumTimeBefore
				scenario.Steps = scenario.Steps[:i+1]
				break
			} else if scenario.Steps[i].Action != DurationAction {
				lastPeriodCoeff := 0
				if scenario.Steps[i].TotalUsersCount%scenario.Steps[i].CountUsersByPeriod != 0 {
					lastPeriodCoeff++
				}
				if sumTimeBefore+scenario.Steps[i].Period*
					float64(scenario.Steps[i].TotalUsersCount/scenario.Steps[i].CountUsersByPeriod+lastPeriodCoeff) > totalDuration {
					delta := totalDuration - sumTimeBefore
					scenario.Steps[i].TotalUsersCount = int(delta/scenario.Steps[i].Period) * scenario.Steps[i].CountUsersByPeriod
				}
			}
			sumTimeBefore += scenario.Steps[i].Period
		}
	}

	scenario.stopUserChannel = make(chan struct{})
	scenario.stopScenarioChannel = make(chan struct{})
	scenario.runningUserWait = &sync.WaitGroup{}
}

func (scenario *Scenario) StartUsersContinually(totalCount int, countByPeriod int, periodInMillis int, testName string, testRunId string) {
	for i := 0; i < totalCount; i += countByPeriod {
		scenario.StartUsers(countByPeriod, testName, testRunId)
		time.Sleep(time.Duration(periodInMillis) * time.Millisecond)
	}
}

func (scenario *Scenario) StartUsers(count int, testName string, testRunId string) {
	for i := 0; i < count; i++ {
		go func() {
			if !scenario.stopped.Load() {
				scenario.runningUserWait.Add(1)
				scenario.StartUser(testName, testRunId)
				scenario.runningUserWait.Done()
			}
		}()
	}
}

func (scenario *Scenario) Run(testName string, testRunId string) int64 {
	startTime := time.Now().Unix()

	for _, step := range scenario.Steps {

		if scenario.stopped.Load() {
			break
		}

		if step.Action == StartAction {
			scenario.StartUsersContinually(step.TotalUsersCount, step.CountUsersByPeriod, int(step.Period*1000), testName, testRunId)
		} else if step.Action == DurationAction {
			select {
			case _ = <-scenario.stopScenarioChannel:
				continue
			case <-time.After(time.Duration(step.Period*1000) * time.Millisecond):
				continue
			}

		} else if step.Action == StopAction {
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
				scenario.stopUserChannel <- struct{}{}
				stopUsersDone.Done()
			}
		}()

		time.Sleep(time.Duration(periodInMillis) * time.Millisecond)
	}
	stopUsersDone.Wait()
}

func (scenario *Scenario) StartUser(testName string, testRunId string) {
	usersCountMetric.WithLabelValues(testName, scenario.Name).Inc()
	user := CreateUser(scenario.Script)
	for {
		timeBeforeIteration := time.Now().UnixMilli()
		result := scenario.Script.ProcessHttp(testName, testRunId, user)
		if result {
			successScenarioCountMetric.WithLabelValues(testName, scenario.Name).Observe(float64(time.Now().UnixMilli()-timeBeforeIteration) / 1000.0)
		} else {
			failedScenarioCountMetric.WithLabelValues(testName, scenario.Name).Inc()
		}
		currentPacing := ((user.userRand.Float64()*2-1)*scenario.PacingDelta + 1) * scenario.Pacing
		timeToSleep := int64(currentPacing*1000) - time.Now().UnixMilli() + timeBeforeIteration
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
