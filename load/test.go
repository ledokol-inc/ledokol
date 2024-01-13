package load

import (
	"net/http"
	"sync"
	"time"
)

type Test struct {
	Id        string
	Name      string
	Scenarios []*Scenario
	Options   *TestOptions
}

type TestOptions struct {
	TotalDuration float64
}

func (test *Test) PrepareTest() {
	for i := range test.Scenarios {
		test.Scenarios[i].PrepareScenario(test.Options.TotalDuration)
		for _, step := range test.Scenarios[i].Script.Steps {
			step.httpClient = &http.Client{
				Timeout: time.Duration(step.Timeout) * time.Millisecond,
			}
		}
	}
}

func (test *Test) Run() {
	testWait := &sync.WaitGroup{}
	for i := range test.Scenarios {
		testWait.Add(1)
		go func(scenario *Scenario, testName string, testRunId string) {
			scenario.Run(testName, testRunId)
			testWait.Done()
		}(test.Scenarios[i], test.Name, test.Id)
	}
	testWait.Wait()
}

func (test *Test) Stop() {
	for i := range test.Scenarios {
		go func(scenario *Scenario) {
			scenario.Stop()
		}(test.Scenarios[i])
	}
}

func (test *Test) SetOptions(options *TestOptions) {
	test.Options = options
}
