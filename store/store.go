package store

import (
	"ledokol/load"
	"time"
)

type Store interface {
	FindTest(name string) (*load.Test, error)
	FindScenarios(name string) ([]load.Scenario, error)
	InsertTest(id int, startTime int64, endTime int64) error
	FindNextTestId() (int, error)
	FindTestTimeFromHistory(string) (time.Time, time.Time, error)
	FindAllTestsFromHistory() ([]TestQuery, error)
}

type InternalError struct {
	err error
}

func (internalErr *InternalError) Error() string {
	return internalErr.err.Error()
}

type NotFoundError struct {
	err error
}

func (notFoundErr *NotFoundError) Error() string {
	return notFoundErr.err.Error()
}

type TestQuery struct {
	Id        string
	StartTime string
	EndTime   string
}
