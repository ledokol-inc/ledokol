package kafkah

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
	"sync"
	"time"
)

type ConsumerWrapper struct {
	consumer    *kafka.Reader
	messages    *sync.Map
	messagesAge *sync.Map
}

func NewConsumer() *ConsumerWrapper {
	return &ConsumerWrapper{consumer: kafka.NewReader(kafka.ReaderConfig{
		Brokers: address,
		GroupID: consumerGroupID,
		Topic:   consumeTopic,
	}), messages: new(sync.Map), messagesAge: new(sync.Map)}
}

func (consumerWrapper *ConsumerWrapper) ProcessConsume() {
	for {
		message, err := consumerWrapper.consumer.ReadMessage(context.Background())
		if err != nil {
			fmt.Println("Произошла ошибка при чтении")
			break
		}
		messageIds := messageIdPattern.FindSubmatch(message.Value)
		if len(messageIds) < 2 {
			fmt.Println("Получено сообщение без messageID")
			continue
		}
		//fmt.Println(message.Headers[1].)
		consumerWrapper.messages.Store(string(messageIds[1]), string(message.Value))
		consumerWrapper.messagesAge.Store(string(messageIds[1]), time.Now().Second())
	}
}

func (consumerWrapper *ConsumerWrapper) Close() {
	if err := consumerWrapper.consumer.Close(); err != nil {
		log.Fatal("failed to close reader:", err)
	} else {
		fmt.Println("Закрыли консумер")
	}
}

func (consumerWrapper *ConsumerWrapper) WaitForMessage(messageId string, timeout int) string {
	startTime := time.Now().UnixMilli()
	for time.Now().UnixMilli()-startTime < int64(timeout) {
		if msg, isFound := consumerWrapper.messages.Load(messageId); isFound {
			consumerWrapper.messages.Delete(messageId)
			return msg.(string)
		}
		time.Sleep(100 * time.Millisecond)
	}
	return ""
}

func (consumerWrapper *ConsumerWrapper) DeleteOldMessages(timeout int) {
	for {
		currentTime := time.Now().Second()
		consumerWrapper.messagesAge.Range(func(key, value interface{}) bool {
			if currentTime-value.(int) > timeout {
				consumerWrapper.messagesAge.Delete(key)
			}
			return true
		})
		time.Sleep(time.Duration(timeout) * time.Second)
	}
}
