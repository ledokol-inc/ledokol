package kafkah

import (
	"fmt"
	"github.com/segmentio/kafka-go"
	"regexp"
)

var consumeTopic = "toDP"
var produceTopic = "invest_pifbuy"

var headers = []kafka.Header{{"nlp-trace-id", []byte("555")}, {"kafka_correlationId", []byte("112")},
	{"X-dynaTrace", []byte("fsdfsdfds")}, {"dp_callback_id", []byte("534")},
	{"dp_publish_time", []byte("29.12.1993")}, {"app_callback_id", []byte("756432")}}

var address = []string{"10.55.24.42:9092", "10.55.24.114:9092"}
var consumerGroupID = "21"

var messageIdPattern = regexp.MustCompile(`"messageId": (.+?),`)
var sessionIdPattern = regexp.MustCompile(`"sessionId": "(.+?),"`)
var userIdPattern = regexp.MustCompile(`"userId": "(.+?),"`)

func ReplaceAll(message []byte, messageId int64, sessionId int64, userId int64) string {
	res := messageIdPattern.ReplaceAllString(string(message), fmt.Sprintf("\"messageId\": %d,", messageId))
	res = sessionIdPattern.ReplaceAllString(res, fmt.Sprintf("\"sessionId\": \"%d\",", sessionId))
	return userIdPattern.ReplaceAllString(res, fmt.Sprintf("\"userId\": \"%d\",", userId))
}
