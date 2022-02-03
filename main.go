package main

import (
	"encoding/csv"
	"errors"
	"fmt"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"ledokol/load"
	"log"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"
)

func main() {

	srv := &http.Server{Addr: ":1454"}
	http.Handle("/metrics", promhttp.Handler())
	http.HandleFunc("/", runTestController)

	err := srv.ListenAndServe()
	log.Fatalf("ListenAndServe(): %v", err)
}

func runTestController(w http.ResponseWriter, r *http.Request) {
	uri := strings.SplitN(r.URL.Path, "/", 4)
	if uri[1] != "test" {
		return
	}
	if uri[2] == "get" {
		getTestTimeFromHistory(uri[3], w)
	} else if uri[2] == "run" {
		test, err := load.InitTestFromFile("res/tests/" + uri[3] + ".json")
		if err != nil {
			w.WriteHeader(http.StatusNotFound)
			w.Write([]byte(err.Error()))
		} else {
			historyFile, err := os.Open(load.TestHistoryFileName)
			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Не найден файл с историей тестов"))
				return
			}
			defer historyFile.Close()
			testId, err := generateTestId(historyFile)

			if err != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte(err.Error()))
				return
			}

			w.Write([]byte("Тест запущен. ID теста - " + strconv.Itoa(testId)))
			go test.Run(testId)
		}
	}
}

func getTestTimeFromHistory(id string, w http.ResponseWriter) {
	historyFile, err := os.Open(load.TestHistoryFileName)
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Не найден файл с историей тестов"))
		return
	}
	defer historyFile.Close()
	csvReader := csv.NewReader(historyFile)
	records, err := csvReader.ReadAll()
	if err != nil {
		w.WriteHeader(http.StatusInternalServerError)
		w.Write([]byte("Неправильный формат данных в файле истории"))
		return
	}
	for i := 1; i < len(records); i++ {
		if records[i][0] == id {
			start, err1 := strconv.ParseInt(records[i][1], 10, 64)
			end, err2 := strconv.ParseInt(records[i][2], 10, 64)
			if err1 != nil || err2 != nil {
				w.WriteHeader(http.StatusInternalServerError)
				w.Write([]byte("Неправильный формат данных в файле истории"))
				return
			}
			fmt.Fprintf(w, "{\"dt_from\": \"%s\", \"dt_to\": \"%s\"}",
				time.Unix(start, 0).Format(load.TimeFormat),
				time.Unix(end, 0).Format(load.TimeFormat))
			return
		}
	}
	w.WriteHeader(http.StatusNotFound)
	w.Write([]byte("Тест с таким id не найден"))
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
