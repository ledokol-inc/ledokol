package load

import "ledokol/kafkah"

type User struct {
	producer *kafkah.ProducerWrapper
}

func NewUser() *User {
	return &User{
		producer: kafkah.NewProducer(),
	}
}

func (user *User) init() {

}

func (user *User) action() {

}

func (user *User) end() {
	user.producer.Close()
}
