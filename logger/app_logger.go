package logger

import (
	"time"

	"github.com/gin-gonic/gin"
	"github.com/rs/zerolog"
	"github.com/rs/zerolog/log"
)

type ginHands struct {
	Path       string
	Latency    time.Duration
	Method     string
	StatusCode int
	ClientIP   string
	MsgStr     string
}

func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		t := time.Now()
		path := c.Request.URL.Path
		raw := c.Request.URL.RawQuery
		c.Next()

		if raw != "" {
			path = path + "?" + raw
		}
		msg := c.Errors.String()
		if msg == "" {
			msg = "Request"
		}
		cData := &ginHands{
			Path:       path,
			Latency:    time.Since(t),
			Method:     c.Request.Method,
			StatusCode: c.Writer.Status(),
			ClientIP:   c.ClientIP(),
			MsgStr:     msg,
		}

		logSwitch(cData)
	}
}

func logSwitch(data *ginHands) {

	var logEvent *zerolog.Event

	switch {
	case data.StatusCode >= 400:
		{
			logEvent = log.Error()
		}
	default:
		logEvent = log.Info()
	}
	logEvent.Str("method", data.Method).Str("path", data.Path).Dur("resp_time", data.Latency).Int("status", data.StatusCode).Str("client_ip", data.ClientIP).Msg(data.MsgStr)
}
