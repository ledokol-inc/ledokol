package load

type Scenario interface {
	PrepareScenario(totalDuration float64)
	Run(testName string) int64
	StartUser(testName string)
	GetName() string
}

type SimpleScenario struct {
	Name            string
	stopUserChannel chan bool
	Steps           []ScenarioStep
	Pacing          float64
	PacingDelta     float64
	Script          *Script
}

type ScenarioStep struct {
	Action             string
	TotalUsersCount    int
	CountUsersByPeriod int
	Period             float64
}
