package load

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
	//runningUsers := &sync.WaitGroup{}
	for i := range test.Scenarios {
		go func(scenario *Scenario, testName string) {
			//usersCountMetric.WithLabelValues(testName, scenario.Name).Set(0)
			scenario.Run(testName)
		}(test.Scenarios[i], test.Name)
	}
}

func (test *Test) Stop() {
	for i := range test.Scenarios {
		go func(scenario *Scenario) {
			scenario.Stop()
		}(test.Scenarios[i])
	}
}
