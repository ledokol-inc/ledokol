package main

import (
	"encoding/csv"
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"ledokol/load"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {

	router := gin.Default()

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/test/history", getAllTestsFromHistory)
	router.GET("/test/history/:id", getTestTimeFromHistory)
	router.POST("/test/catalog/:name", func(c *gin.Context) {
		name := c.Param("name")
		action := c.Query("action")
		if action == "run" {
			test, err := load.InitTestFromFile("res/tests/" + name + ".json")
			if err != nil {
				c.String(http.StatusNotFound, err.Error())
			} else {
				historyFile, err := os.Open(load.TestHistoryFileName)
				if err != nil {
					c.String(http.StatusInternalServerError, "Не найден файл с историей тестов")
					return
				}
				defer historyFile.Close()
				testId, err := generateTestId(historyFile)

				if err != nil {
					c.String(http.StatusInternalServerError, err.Error())
					return
				}

				c.String(http.StatusOK, "Тест запущен. ID теста - "+strconv.Itoa(testId))
				go test.Run(testId)
			}
		}
	})
	router.GET("/test/history/pc/:id", getPCCompatibleTestInfo)

	err := router.Run(":1454")
	log.Fatalf("ListenAndServe(): %v", err)
}

func getPCCompatibleTestInfo(context *gin.Context) {
	id := context.Param("id")
	historyFile, err := os.Open(load.TestHistoryFileName)
	if err != nil {
		context.String(http.StatusInternalServerError, "Не найден файл с историей тестов")
		return
	}
	defer historyFile.Close()
	csvReader := csv.NewReader(historyFile)
	records, err := csvReader.ReadAll()
	if err != nil {
		context.String(http.StatusInternalServerError, "Неправильный формат данных в файле истории")
		return
	}
	for i := 1; i < len(records); i++ {
		if records[i][0] == id {
			start, err1 := strconv.ParseInt(records[i][1], 10, 64)
			end, err2 := strconv.ParseInt(records[i][2], 10, 64)
			if err1 != nil || err2 != nil {
				context.String(http.StatusInternalServerError, "Неправильный формат данных в файле истории")
				return
			}
			context.String(http.StatusOK, "{\"dt_from\": \"%s\", \"dt_to\": \"%s\"}",
				time.Unix(start, 0).Format(load.TimeFormat),
				time.Unix(end, 0).Format(load.TimeFormat))
			return
		}
	}
	context.String(http.StatusNotFound, "Тест с таким id не найден")
}

func getTestTimeFromHistory(context *gin.Context) {
	id := context.Param("id")
	historyFile, err := os.Open(load.TestHistoryFileName)
	if err != nil {
		context.String(http.StatusInternalServerError, "Не найден файл с историей тестов")
		return
	}
	defer historyFile.Close()
	csvReader := csv.NewReader(historyFile)
	records, err := csvReader.ReadAll()
	if err != nil {
		context.String(http.StatusInternalServerError, "Неправильный формат данных в файле истории")
		return
	}
	for i := 1; i < len(records); i++ {
		if records[i][0] == id {
			start, err1 := strconv.ParseInt(records[i][1], 10, 64)
			end, err2 := strconv.ParseInt(records[i][2], 10, 64)
			if err1 != nil || err2 != nil {
				context.String(http.StatusInternalServerError, "Неправильный формат данных в файле истории")
				return
			}
			context.JSON(http.StatusOK, testQuery{id, time.Unix(start, 0).Format(load.TimeFormat),
				time.Unix(end, 0).Format(load.TimeFormat)})
			return
		}
	}
	context.String(http.StatusNotFound, "Тест с таким id не найден")
}

func getAllTestsFromHistory(context *gin.Context) {
	var tests []testQuery
	historyFile, err := os.Open(load.TestHistoryFileName)
	if err != nil {
		context.String(http.StatusInternalServerError, "Не найден файл с историей тестов")
		return
	}
	defer historyFile.Close()
	csvReader := csv.NewReader(historyFile)
	records, err := csvReader.ReadAll()
	if err != nil {
		context.String(http.StatusInternalServerError, "Неправильный формат данных в файле истории")
		return
	}
	for i := 1; i < len(records); i++ {
		start, err1 := strconv.ParseInt(records[i][1], 10, 64)
		end, err2 := strconv.ParseInt(records[i][2], 10, 64)
		if err1 != nil || err2 != nil {
			context.String(http.StatusInternalServerError, "Неправильный формат данных в файле истории")
			return
		}
		tests = append(tests, testQuery{records[i][0], time.Unix(start, 0).Format(load.TimeFormat),
			time.Unix(end, 0).Format(load.TimeFormat)})
	}
	context.JSON(http.StatusOK, tests)
}

type testQuery struct {
	Id        string
	StartTime string
	EndTime   string
}

func generateTestId(historyFile *os.File) (int, error) {
	csvReader := csv.NewReader(historyFile)
	records, err := csvReader.ReadAll()
	if err != nil {
		return 0, errors.New("Неправильный формат данных в файле истории")
	}
	testId := 1
	for i := 1; i < len(records); i++ {
		id, err := strconv.Atoi(records[i][0])
		if err != nil {
			return 0, errors.New("Неправильный формат данных в файле истории")
		}
		if id > testId {
			testId = id
		}
	}
	testId++
	return testId, nil
}
