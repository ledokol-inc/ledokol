package main

import (
	"errors"
	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus/promhttp"
	"io"
	"ledokol/load"
	"ledokol/store"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"
)

func main() {

	storeObj := store.NewBoltStore("ledokol.db")

	serverLogFile, _ := os.Create("server.log")

	gin.DefaultWriter = io.MultiWriter(serverLogFile, os.Stdout)
	gin.DefaultErrorWriter = io.MultiWriter(serverLogFile, os.Stdout)

	router := gin.Default()

	router.GET("/metrics", gin.WrapH(promhttp.Handler()))
	router.GET("/test/history", getAllTestsFromHistory(storeObj))
	router.GET("/test/history/:id", getTestTimeFromHistory(storeObj))
	router.GET("/test/catalog", getAllTestsFromCatalog(storeObj))
	router.POST("/test/catalog", insertTestInCatalog(storeObj))
	router.POST("/test/catalog/:name", func(c *gin.Context) {
		name := c.Param("name")
		action := c.Query("action")
		if action == "run" {

			testId, err := runTest(name, storeObj)

			if err != nil {
				processMiddlewareError(c, err)
			} else {
				c.String(http.StatusOK, "Тест запущен. ID теста - "+strconv.FormatInt(testId, 10))
			}
		} else if action == "stop" {
			//TODO
		}
	})
	router.GET("/test/history/pc/:id", getPCCompatibleTestInfo(storeObj))

	err := router.Run(":1454")
	log.Fatalf("ListenAndServe(): %v", err)
}

func getPCCompatibleTestInfo(storeObj store.Store) gin.HandlerFunc {

	return func(context *gin.Context) {
		id := context.Param("id")
		start, end, err := storeObj.FindTestTimeFromHistory(id)

		processMiddlewareError(context, err)
		context.String(http.StatusOK, "{\"dt_from\": \"%s\", \"dt_to\": \"%s\"}",
			start.Format(load.TimeFormat),
			end.Format(load.TimeFormat))
	}
}

func getTestTimeFromHistory(storeObj store.Store) gin.HandlerFunc {

	return func(context *gin.Context) {
		id := context.Param("id")

		start, end, err := storeObj.FindTestTimeFromHistory(id)

		processMiddlewareError(context, err)
		context.JSON(http.StatusOK, store.TestQuery{Id: id, StartTime: start.Format(load.TimeFormat),
			EndTime: end.Format(load.TimeFormat)})
	}
}

func getAllTestsFromHistory(storeObj store.Store) gin.HandlerFunc {

	return func(context *gin.Context) {
		tests, err := storeObj.FindAllTestsFromHistory()

		processMiddlewareError(context, err)

		context.JSON(http.StatusOK, tests)
	}
}

func getAllTestsFromCatalog(storeObj store.Store) gin.HandlerFunc {

	return func(context *gin.Context) {
		tests, err := storeObj.FindAllTestsFromCatalog()

		processMiddlewareError(context, err)

		context.JSON(http.StatusOK, tests)
	}
}

func insertTestInCatalog(storeObj store.Store) gin.HandlerFunc {

	return func(context *gin.Context) {
		var test load.Test
		err := context.BindJSON(&test)
		if err == nil {
			err = storeObj.InsertTestInCatalog(&test)
		}

		if err != nil {
			processMiddlewareError(context, err)
		}
	}
}

func processMiddlewareError(context *gin.Context, err error) {
	if err != nil {
		var internalErr *store.InternalError
		if errors.As(err, &internalErr) {
			context.String(http.StatusInternalServerError, err.Error())
			return
		}
		var notFoundErr *store.NotFoundError
		if errors.As(err, &notFoundErr) {
			context.String(http.StatusNotFound, err.Error())
			return
		}
	}
}

func runTest(name string, store store.Store) (int64, error) {
	test, err := store.FindTest(name)

	if err != nil {
		return 0, err
	}

	load.PrepareTest(test)

	startTime := time.Now().Unix()

	testId, err := store.InsertTest(startTime, startTime)

	if err != nil {
		return 0, err
	}

	go func(testId int64) {
		test.Run(testId)
		//TODO Update time and status running store.InsertTest(testId, startTime, time.Now().Unix())
	}(testId)

	return testId, nil
}
