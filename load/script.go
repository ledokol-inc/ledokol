package load

import (
	"bytes"
	"encoding/base64"
	"io"
	"math/rand"
	"net/http"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"

	"github.com/ledokol-inc/ledokol/load/variables"
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

func (script *Script) ProcessHttp(testName string, testRunId string, user *User) bool {
	success := true
	iter := script.prepareIteration(user, testRunId)

	for _, step := range script.Steps {
		var req *http.Request
		var err error

		var resultMessage string
		if step.Message == "" {
			req, err = http.NewRequest(step.Method, step.Url, nil)
		} else {
			resultMessage = script.prepareStep(user, iter, step)
			req, err = http.NewRequest(step.Method, step.Url, bytes.NewBufferString(resultMessage))
		}

		if err != nil {
			beginLogInScript(true, err, iter, step.Name).Msgf("Не удалось создать объект запроса")
			break
		}

		for key, value := range step.Headers {
			req.Header.Set(key, value)
		}

		requestId := randomId(user.userRand, requestIdLength)
		beginLogInScript(false, nil, iter, step.Name).
			Str("body", resultMessage).Str("requestId", requestId).Msg("Отправка запроса")

		startTime := time.Now().UnixMilli()
		resp, err := step.httpClient.Do(req)

		if err != nil || resp.StatusCode >= 300 {
			failedTransactionCountMetric.WithLabelValues(testName, script.Name, step.Name, "true").Inc()
			success = false
			if err != nil {
				beginLogInScript(true, err, iter, step.Name).
					Str("requestId", requestId).Msg("Ошибка отправки запроса")
			} else {
				body, err := getResponseBody(resp)
				if err != nil {
					logReadResponseError(err, iter, step.Name, resp.StatusCode, requestId)
				} else {
					logReadResponse(iter, step.Name, resp.StatusCode, requestId, body, true)
				}
			}
			break
		} else {
			successTransactionCountMetric.WithLabelValues(testName, script.Name, step.Name).
				Observe(float64(time.Now().UnixMilli()-startTime) / 1000.0)

			body, err := getResponseBody(resp)
			if err != nil {
				logReadResponseError(err, iter, step.Name, resp.StatusCode, requestId)
			} else {
				logReadResponse(iter, step.Name, resp.StatusCode, requestId, body, false)
			}
		}
	}

	return success
}

func (script *Script) prepareIteration(user *User, testRunId string) *iteration {
	script.generateVariablesForStage(user, variables.IterationScope)
	return &iteration{
		testRunId:  testRunId,
		scriptName: script.Name,
		userId:     user.id,
		id:         randomId(user.userRand, iterationIdLength),
	}
}

func (script *Script) prepareStep(user *User, iter *iteration, step *Step) string {
	script.generateVariablesForStage(user, variables.StepScope)

	var replaces replaceSlice
	for name := range user.scriptVariables {
		indexes := script.Variables[name].InsertingRegex.FindStringSubmatchIndex(step.Message)
		if len(indexes) < 4 {
			beginLogInScript(true, nil, iter, step.Name).
				Str("variable", name).Msgf("Не найдена группа для замены в сообщении")
		} else {
			replaces = append(replaces, replaceInfo{start: indexes[2], end: indexes[3], varName: name})
		}
	}

	if replaces != nil {
		replaces.sortByStartIndex()
		return replaceByIndexes(step.Message, replaces, user.scriptVariables)
	} else {
		return step.Message
	}
}

func (script *Script) generateVariablesForStage(user *User, stage variables.Scope) {
	for name, variable := range script.Variables {
		if variable.Scope == stage {
			user.scriptVariables[name] = variable.Generate(user.userRand)
		}
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

func logReadResponseError(err error, iter *iteration, stepName string, status int, requestId string) {

	beginLogInScript(true, err, iter, stepName).Str("status", strconv.Itoa(status)).
		Str("requestId", requestId).Msg("При попытке чтения ответа произошла ошибка")
}

func logReadResponse(iter *iteration, stepName string, status int, requestId string, body string, isError bool) {
	beginLogInScript(isError, nil, iter, stepName).Str("body", body).Str("status", strconv.Itoa(status)).
		Str("requestId", requestId).Msg("Получен ответ")
}

func beginLogInScript(isError bool, err error, iter *iteration, stepName string) *zerolog.Event {
	var event *zerolog.Event
	if isError {
		event = log.Error()
		if err != nil {
			event = event.Err(err)
		}
	} else {
		event = log.Info()
	}

	return event.Str("testRunId", iter.testRunId).Str("script", iter.scriptName).
		Str("userId", iter.userId).Str("iterationId", iter.id).Str("step", stepName)
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
