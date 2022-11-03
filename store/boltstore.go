package store

import (
	"errors"
	"github.com/asdine/storm/v3"
	"ledokol/load"
	"strconv"
	"time"
)

type BoltStore struct {
	dbName string
}

func NewBoltStore(dbName string) *BoltStore {
	return &BoltStore{dbName}
}

func (store *BoltStore) FindTest(name string) (*load.Test, error) {

	db, err := storm.Open(store.dbName)
	if err == nil {
		defer db.Close()
		testBucket := db.From("Tests")
		var test load.Test
		err = testBucket.One("Name", name, &test)
		if err != nil {
			return nil, &InternalError{err}
		}

		return &test, nil
	} else {
		return nil, &InternalError{errors.New("Не удалось открыть базу данных")}
	}
}

func (store *BoltStore) FindAllTestsFromCatalog() ([]load.Test, error) {

	db, err := storm.Open(store.dbName)
	if err == nil {
		defer db.Close()
		testBucket := db.From("Tests")
		var tests []load.Test
		err = testBucket.All(&tests)
		if err != nil {
			return nil, &InternalError{err}
		}
		return tests, nil
	} else {
		return nil, &InternalError{errors.New("Не удалось открыть базу данных")}
	}
}

func (store *BoltStore) InsertTestInCatalog(test *load.Test) error {
	db, err := storm.Open(store.dbName)
	if err == nil {
		testBucket := db.From("Tests")
		err = testBucket.Save(&test)

		if err1 := db.Close(); err1 != nil {
			return &InternalError{errors.New("Не удалось закрыть базу данных")}
		}

		if err != nil {
			return &InternalError{err}
		}

		return nil
	} else {
		return &InternalError{errors.New("Не удалось открыть базу данных")}
	}
}

func (store *BoltStore) InsertTest(startTime int64, endTime int64) (int64, error) {
	db, err := storm.Open(store.dbName)
	if err == nil {
		historyBucket := db.From("History")
		historyRaw := &testHistoryRaw{StartTime: startTime, EndTime: endTime}
		err = historyBucket.Save(&historyRaw)

		if err1 := db.Close(); err1 != nil {
			return -1, &InternalError{errors.New("Не удалось закрыть базу данных")}
		}

		if err != nil {
			return -1, &InternalError{err}
		}

		return historyRaw.ID, nil
	} else {
		return -1, &InternalError{errors.New("Не удалось открыть базу данных")}
	}
}

func (store *BoltStore) FindAllTestsFromHistory() ([]TestQuery, error) {
	db, err := storm.Open(store.dbName)
	if err == nil {
		historyBucket := db.From("History")
		var historyRaws []testHistoryRaw
		err := historyBucket.All(&historyRaws)

		if err1 := db.Close(); err1 != nil {
			return nil, &InternalError{errors.New("Не удалось закрыть базу данных")}
		}

		if err != nil {
			return nil, &InternalError{err}
		}

		result := make([]TestQuery, 0)

		for i := 0; i < len(historyRaws); i++ {
			result = append(result, TestQuery{Id: strconv.FormatInt(historyRaws[i].ID, 10),
				StartTime: time.Unix(historyRaws[i].StartTime, 0).Format(load.TimeFormat),
				EndTime:   time.Unix(historyRaws[i].EndTime, 0).Format(load.TimeFormat)})
		}
		return result, nil
	} else {
		return nil, &InternalError{errors.New("Не удалось открыть базу данных")}
	}
}

func (store *BoltStore) FindTestTimeFromHistory(id string) (time.Time, time.Time, error) {
	db, err := storm.Open(store.dbName)
	if err == nil {
		historyBucket := db.From("History")
		var historyRaw testHistoryRaw
		err := historyBucket.One("ID", id, &historyRaw)

		if err1 := db.Close(); err1 != nil {
			return time.Time{}, time.Time{}, &InternalError{errors.New("Не удалось закрыть базу данных")}
		}

		if err != nil {
			return time.Time{}, time.Time{}, &InternalError{err}
		}

		return time.Unix(historyRaw.StartTime, 0), time.Unix(historyRaw.EndTime, 0), nil
	} else {
		return time.Time{}, time.Time{}, &InternalError{errors.New("Не удалось открыть базу данных")}
	}
}

type testHistoryRaw struct {
	ID        int64 `storm:"id,increment"`
	StartTime int64
	EndTime   int64
}
