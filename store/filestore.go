package store

import (
	"encoding/csv"
	"encoding/json"
	"errors"
	"fmt"
	"ledokol/load"
	"os"
	"strconv"
	"time"
)

type FileStore struct {
	resPath             string
	TestHistoryFileName string
}

func NewFileStore(resPath string) *FileStore {
	return &FileStore{resPath: resPath, TestHistoryFileName: resPath + "/tests/test_history.csv"}
}

func (store *FileStore) FindTest(name string) (*load.Test, error) {
	result := new(load.Test)
	data, err := os.ReadFile(name)
	if err != nil {
		return nil, &NotFoundError{errors.New("Файл с описанием теста не найден")}
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, &InternalError{errors.New("Некорректное описание теста")}
	}

	result.Scenarios, err = store.FindScenarios(result.ScenariosName)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func (store *FileStore) FindScenarios(name string) ([]load.Scenario, error) {
	result := make([]load.Scenario, 0)
	data, err := os.ReadFile(fmt.Sprintf("%s/scenarios/%s/%s.json", store.resPath, name, name))
	if err != nil {
		return nil, &NotFoundError{errors.New("Файл со сценариями для теста не найден")}
	}
	err = json.Unmarshal(data, &result)
	if err != nil {
		return nil, &InternalError{errors.New("Файл сценариев для теста имеет неверный формат\nError: " + err.Error())}
	}
	for i := range result {
		for j := range result[i].Steps {
			if result[i].Steps[j].FileName != "" {
				var messageInBytes []byte
				messageInBytes, err = os.ReadFile(fmt.Sprintf("%s/scenarios/%s/messages/%s", store.resPath, name, result[i].Steps[j].FileName))
				result[i].Steps[j].Message = string(messageInBytes)
			}
			if err != nil {
				return nil, &NotFoundError{errors.New(fmt.Sprintf("Файл с сообщением для сценария \"%s\" шага \"%s\" не найден", result[i].Name, result[i].Steps[j].Name))}
			}
		}
	}
	return result, nil
}

func (store *FileStore) InsertTest(id int, startTime int64, endTime int64) error {
	historyFile, err := os.OpenFile(store.TestHistoryFileName, os.O_WRONLY|os.O_APPEND, 0660)
	if err != nil {
		writer := csv.NewWriter(historyFile)
		err := writer.Write([]string{strconv.Itoa(id), strconv.FormatInt(time.Unix(startTime, 0).Unix(), 10),
			strconv.FormatInt(time.Unix(endTime, 0).Unix(), 10)})
		if err != nil {
			historyFile.Close()
			return &InternalError{errors.New("Не удалось записать изменения в файл истории")}
		}
		writer.Flush()
	} else {
		historyFile.Close()
		return &InternalError{errors.New("Не удалось открыть файл истории")}
	}

	if err = historyFile.Close(); err != nil {
		return &InternalError{errors.New("Не удалось закрыть файл истории")}
	}

	return nil
}

func (store *FileStore) FindNextTestId() (int, error) {

	records, err := store.getRecordsFromTestFile()

	if err != nil {
		return 0, err
	}

	testId := 1
	for i := 1; i < len(records); i++ {
		id, err := strconv.Atoi(records[i][0])
		if err != nil {
			return 0, &InternalError{errors.New("Неправильный формат данных в файле истории")}
		}
		if id > testId {
			testId = id
		}
	}
	testId++
	return testId, nil
}

func (store *FileStore) FindTestTimeFromHistory(id string) (time.Time, time.Time, error) {
	records, err := store.getRecordsFromTestFile()

	if err != nil {
		return time.Time{}, time.Time{}, err
	}

	for i := 1; i < len(records); i++ {
		if records[i][0] == id {
			start, err1 := strconv.ParseInt(records[i][1], 10, 64)
			end, err2 := strconv.ParseInt(records[i][2], 10, 64)
			if err1 != nil || err2 != nil {
				return time.Time{}, time.Time{}, &InternalError{errors.New("Неправильный формат данных в файле истории")}
			}
			return time.Unix(start, 0), time.Unix(end, 0), nil
		}
	}
	return time.Time{}, time.Time{}, &NotFoundError{errors.New("Тест с таким id не найден")}
}

func (store *FileStore) FindAllTestsFromHistory() ([]TestQuery, error) {
	var tests []TestQuery
	records, err := store.getRecordsFromTestFile()

	if err != nil {
		return nil, err
	}

	for i := 1; i < len(records); i++ {
		start, err1 := strconv.ParseInt(records[i][1], 10, 64)
		end, err2 := strconv.ParseInt(records[i][2], 10, 64)
		if err1 != nil || err2 != nil {
			return nil, &InternalError{errors.New("Неправильный формат данных в файле истории")}
		}
		tests = append(tests, TestQuery{records[i][0], time.Unix(start, 0).Format(load.TimeFormat),
			time.Unix(end, 0).Format(load.TimeFormat)})
	}

	return tests, nil
}

func (store *FileStore) getRecordsFromTestFile() ([][]string, error) {
	historyFile, err := os.Open(store.TestHistoryFileName)
	if err != nil {
		return nil, &InternalError{errors.New("Не найден файл с историей тестов")}
	}
	defer historyFile.Close()
	csvReader := csv.NewReader(historyFile)
	records, err := csvReader.ReadAll()
	if err != nil {
		return nil, &InternalError{errors.New("Неправильный формат данных в файле истории")}
	}

	return records, nil
}
