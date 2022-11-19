package load

import "sync"

type Test struct {
	Name          string
	Scenarios     []*Scenario
	TotalDuration float64
	Id            string
}

func (test *Test) PrepareTest() {
	for i := range test.Scenarios {
		test.Scenarios[i].PrepareScenario(test.TotalDuration)
	}
}

func (test *Test) Run() {
	testWait := &sync.WaitGroup{}
	for i := range test.Scenarios {
		testWait.Add(1)
		go func(scenario *Scenario, testName string) {
			scenario.Run(testName)
			testWait.Done()
		}(test.Scenarios[i], test.Name)
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
