package variables

import (
	"math/rand"
	"regexp"
	"regexp/syntax"

	reggen "github.com/ledokol-inc/string-generation"
)

const regexManyCharactersLimit = 10

type Scope string

const (
	IterationScope Scope = "iteration"
	StepScope      Scope = "step"
	ScenarioScope  Scope = "scenario"
)

type Variable struct {
	Name            string
	Scope           Scope
	GenerationRegex *syntax.Regexp `mapstructure:"generationRegex" json:"generationRegex"`
	InsertingRegex  *regexp.Regexp
}

func (variable *Variable) Generate(userRand *rand.Rand) string {
	return reggen.Generate(variable.GenerationRegex, regexManyCharactersLimit, userRand)
}
