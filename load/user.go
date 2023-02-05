package load

import (
	"ledokol/load/variables"
	"math/rand"
)

const userIdLength = 15

type User struct {
	scriptVariables map[string]string
	id              string
	userRand        *rand.Rand
}

func CreateUser(script *Script) *User {

	user := &User{scriptVariables: make(map[string]string), userRand: initRand()}
	user.id = randomId(user.userRand, userIdLength)
	script.generateVariablesForStage(user, variables.ScenarioScope)
	return user
}

func initRand() *rand.Rand {
	return rand.New(rand.NewSource(rand.Int63()))
}
