package kafkah

import (
	"context"
	"fmt"
	"github.com/segmentio/kafka-go"
	"log"
)

type ProducerWrapper struct {
	producer *kafka.Writer
	headers  []kafka.Header
}

func NewProducer() *ProducerWrapper {
	return &ProducerWrapper{producer: &kafka.Writer{
		Addr:  kafka.TCP(address...),
		Topic: produceTopic,
	}, headers: headers}
}

func (producerWrapper *ProducerWrapper) SendMessage(message []byte) {
	err := producerWrapper.producer.WriteMessages(context.Background(),
		kafka.Message{
			Key:     []byte(""),
			Value:   message,
			Headers: producerWrapper.headers,
		},
	)
	if err != nil {
		log.Fatal("failed to write messages:", err)
	}
}

func (producerWrapper *ProducerWrapper) Close() {
	if err := producerWrapper.producer.Close(); err != nil {
		log.Fatal("failed to close writer:", err)
	} else {
		fmt.Println("Закрыли продюсер")
	}
}
