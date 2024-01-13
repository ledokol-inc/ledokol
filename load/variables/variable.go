package variables

import (
	reggen "github.com/ledokol-inc/string-generation"
	"math/rand"
	"regexp"
	"regexp/syntax"
)

const regexManyCharactersLimit = 10

type Scope string

const (
	IterationScope Scope = "iteration"
	StepScope            = "step"
	ScenarioScope        = "scenario"
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
