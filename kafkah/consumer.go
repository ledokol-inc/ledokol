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
	consumer *kafka.Reader
	messages *sync.Map
}

func NewConsumer() *ConsumerWrapper {
	return &ConsumerWrapper{consumer: kafka.NewReader(kafka.ReaderConfig{
		Brokers: address,
		GroupID: consumerGroupID,
		Topic:   consumeTopic,
	}), messages: new(sync.Map)}
}

func (consumerWrapper *ConsumerWrapper) ProcessConsume() {
	for {
		message, err := consumerWrapper.consumer.ReadMessage(context.Background())
		if err != nil {
			fmt.Println("Произошла ошибка при чтении")
		}
		consumerWrapper.messages.Store(string(messageIdPattern.FindSubmatch(message.Value)[1]), string(message.Value))
		/*fmt.Printf("message at topic/partition/offset %v/%v/%v: %s = %s\n", message.Topic, message.Partition,
		message.Offset, string(message.Key), string(message.Value))*/
	}
}

func (consumerWrapper *ConsumerWrapper) Close() {
	if err := consumerWrapper.consumer.Close(); err != nil {
		log.Fatal("failed to close reader:", err)
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
