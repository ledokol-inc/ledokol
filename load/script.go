package load

import (
	"bytes"
	"encoding/base64"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
	"io"
	"ledokol/load/variables"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"
)

const requestIdLength = 15

type Script struct {
	Name      string
	Steps     []*Step
	Variables map[string]*variables.Variable
}

type Step struct {
	Name       string
	Message    string `mapstructure:"body" json:"body"`
	Url        string
	Method     string
	Headers    map[string]string
	httpClient *http.Client
	Timeout    int64
}

func (script *Script) ProcessHttp(testName string, userRand *rand.Rand, scriptVariables map[string]string) (bool, int64) {
	success := true
	startIterationTime := time.Now().UnixMilli()
	script.PrepareIteration(scriptVariables, userRand)

	for _, step := range script.Steps {
		startTime := time.Now().UnixMilli()

		var req *http.Request
		var err error

		var resultMessage string
		if step.Message == "" {
			req, err = http.NewRequest(step.Method, step.Url, nil)
		} else {
			resultMessage = script.PrepareStep(scriptVariables, userRand, testName, step)
			req, err = http.NewRequest(step.Method, step.Url, bytes.NewBufferString(resultMessage))
		}

		if err != nil {
			log.Error().Err(err).Str("test", testName).
				Str("script", script.Name).Str("step", step.Name).
				Msgf("Не удалось создать объект запроса")
			break
		}

		for key, value := range step.Headers {
			req.Header.Set(key, value)
		}

		requestId := randomId(userRand, requestIdLength)

		resp, err := step.httpClient.Do(req)

		log.Info().Str("test", testName).Str("script", script.Name).
			Str("step", step.Name).Str("body", resultMessage).
			Str("requestId", requestId).Msg("Отправка запроса")

		if err != nil || resp.StatusCode >= 300 {
			failedTransactionCountMetric.WithLabelValues(testName, script.Name, step.Name, "true").Inc()
			success = false
			if err != nil {
				log.Error().Err(err).Str("test", testName).
					Str("script", script.Name).Str("step", step.Name).
					Str("requestId", requestId).Msg("Ошибка отправки запроса")
			} else {
				body, err := getResponseBody(resp)
				if err != nil {
					logReadResponseError(err, testName, script.Name, step.Name, resp.StatusCode, requestId)
				} else {
					logReadResponse(err, testName, script.Name, step.Name, resp.StatusCode, requestId, body, true)
				}
			}
			break
		} else {
			successTransactionCountMetric.WithLabelValues(testName, script.Name, step.Name).
				Observe(float64(time.Now().UnixMilli()-startTime) / 1000.0)

			body, err := getResponseBody(resp)
			if err != nil {
				logReadResponseError(err, testName, script.Name, step.Name, resp.StatusCode, requestId)
			} else {
				logReadResponse(err, testName, script.Name, step.Name, resp.StatusCode, requestId, body, false)
			}
		}
	}

	return success, startIterationTime
}

func (script *Script) GenerateVariablesForStage(scriptVariables map[string]string, stage variables.Scope, userRand *rand.Rand) {
	for name, variable := range script.Variables {
		if variable.Scope == stage {
			scriptVariables[name] = variable.Generate(userRand)
		}
	}
}

func (script *Script) PrepareScript(userRand *rand.Rand) map[string]string {
	generatedVars := make(map[string]string)

	script.GenerateVariablesForStage(generatedVars, variables.ScenarioScope, userRand)
	return generatedVars
}

func (script *Script) PrepareIteration(scriptVariables map[string]string, userRand *rand.Rand) {
	script.GenerateVariablesForStage(scriptVariables, variables.IterationScope, userRand)
}

func (script *Script) PrepareStep(scriptVariables map[string]string, userRand *rand.Rand, testName string, step *Step) string {
	script.GenerateVariablesForStage(scriptVariables, variables.StepScope, userRand)

	var replaces replaceSlice
	for name := range scriptVariables {
		indexes := script.Variables[name].InsertingRegex.FindStringSubmatchIndex(step.Message)
		if len(indexes) < 4 {
			log.Error().Str("test", testName).
				Str("script", script.Name).Str("step", step.Name).
				Str("variable", name).Msgf("Не найдена группа для замены в сообщении")
		} else {
			replaces = append(replaces, replaceInfo{start: indexes[2], end: indexes[3], varName: name})
		}
	}

	if replaces != nil {
		replaces.sortByStartIndex()
		return replaceByIndexes(step.Message, replaces, scriptVariables)
	} else {
		return step.Message
	}
}

func getResponseBody(resp *http.Response) (string, error) {
	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	resp.Body.Close()
	return string(body), nil
}

func randomId(userRand *rand.Rand, length int) string {
	b := make([]byte, length)
	userRand.Read(b)

	return base64.RawURLEncoding.EncodeToString(b)
}

func logReadResponseError(err error, testName string, scriptName string, stepName string, status int, requestId string) {
	log.Error().Err(err).Str("test", testName).Str("script", scriptName).
		Str("step", stepName).Str("status", strconv.Itoa(status)).
		Str("requestId", requestId).Msg("При попытке чтения ответа произошла ошибка")
}

func logReadResponse(err error, testName string, scriptName string, stepName string, status int, requestId string, body string, isError bool) {
	var event *zerolog.Event
	if isError {
		event = log.Error()
	} else {
		event = log.Info()
	}
	event.Str("test", testName).Str("script", scriptName).
		Str("step", stepName).Str("body", body).
		Str("status", strconv.Itoa(status)).Str("requestId", requestId).Msg("Получен ответ")
}

type replaceInfo struct {
	start   int
	end     int
	varName string
}

type replaceSlice []replaceInfo

func (rSlice replaceSlice) sortByStartIndex() {
	sort.Slice(rSlice, func(i, j int) bool {
		return rSlice[i].start < rSlice[j].start
	})
}

func replaceByIndexes(str string, replaces replaceSlice, scriptVariables map[string]string) string { // Итого по времени: O(m + n * log(n)) По памяти: O(n + m)
	var result strings.Builder
	var lastEnd int
	for _, replacement := range replaces {
		result.WriteString(str[lastEnd:replacement.start])
		result.WriteString(scriptVariables[replacement.varName])
		lastEnd = replacement.end
	}
	result.WriteString(str[lastEnd:])
	return result.String()
}
