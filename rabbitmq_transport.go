package servicebus

import (
	"errors"
	"github.com/streadway/amqp"
	"log"
)

// RabbitMQClient - клієнт для роботи з RabbitMQ
type RabbitMQClient struct {
	Connection *amqp.Connection
	Channel    *amqp.Channel
	Exchange   string
	Queue      string
	Serializer *JSONSerializer
}

// NewRabbitMQClient - створює нового клієнта RabbitMQ
func NewRabbitMQClient(amqpURL, exchange, queue string) (*RabbitMQClient, error) {
	log.Println("Initializing RabbitMQ connection...")

	conn, err := amqp.Dial(amqpURL)
	if err != nil {
		log.Printf("Failed to connect to RabbitMQ: %v\n", err)
		return nil, err
	}

	log.Println("Connection to RabbitMQ established successfully.")

	ch, err := conn.Channel()
	if err != nil {
		log.Printf("Failed to open a channel: %v\n", err)
		conn.Close()
		return nil, err
	}
	log.Println("Channel opened successfully.")

	serializer := &JSONSerializer{}

	client := &RabbitMQClient{
		Connection: conn,
		Channel:    ch,
		Exchange:   exchange,
		Queue:      queue,
		Serializer: serializer,
	}

	if err := client.createExchange(); err != nil {
		log.Printf("Failed to create exchange: %v\n", err)
		client.closeChanelConnection()
		return nil, err
	}

	if err := client.createQueue(); err != nil {
		log.Printf("Failed to create queue: %v\n", err)
		client.closeChanelConnection()
		return nil, err
	}

	log.Println("RabbitMQ setup completed successfully.")
	return client, nil
}

// closeChanelConnection - закриває канал і з'єднання
func (client *RabbitMQClient) closeChanelConnection() {
	log.Println("Closing RabbitMQ channel and connection...")
	client.Channel.Close()
	client.Connection.Close()
	log.Println("RabbitMQ channel and connection closed.")
}

// createExchange - створює exchange, якщо це необхідно
func (client *RabbitMQClient) createExchange() error {
	if client.Exchange != "" {
		log.Printf("Creating exchange: %s\n", client.Exchange)
		err := client.Channel.ExchangeDeclare(
			client.Exchange,
			"direct",
			true,
			false,
			false,
			false,
			nil)
		if err != nil {
			log.Printf("Failed to declare exchange: %v\n", err)
		}
		return err
	}
	return nil
}

// createQueue - створює чергу, якщо це необхідно
func (client *RabbitMQClient) createQueue() error {
	if client.Queue != "" {
		log.Printf("Creating queue: %s\n", client.Queue)
		_, err := client.Channel.QueueDeclare(
			client.Queue,
			true,
			false,
			false,
			false,
			nil)
		if err != nil {
			log.Printf("Failed to declare queue: %v\n", err)
		}
		return err
	}
	return nil
}

// bindQueueToExchange - прив'язує чергу до exchange з вказаним роутінг ключем
func (client *RabbitMQClient) BindQueueToExchange(routingKey string) error {
	log.Printf("Binding queue %s to exchange %s with routing key %s\n", client.Queue, client.Exchange, routingKey)
	err := client.Channel.QueueBind(
		client.Queue,
		routingKey, // використовуємо динамічний роутінг ключ
		client.Exchange,
		false,
		nil)
	if err != nil {
		log.Printf("Failed to bind queue to exchange: %v\n", err)
	}
	return err
}

// Send - відправляє повідомлення з використанням роутінг ключа
func (client *RabbitMQClient) Send(message Message) error {
	log.Println("Sending message...")

	if client.Connection == nil {
		log.Println("Connection does not exist.")
		return errors.New("connection does not exist")
	}

	if client.Channel == nil {
		log.Println("Channel does not exist.")
		return errors.New("channel does not exist")
	}

	body, err := client.Serializer.Marshal(message)
	if err != nil {
		log.Printf("Failed to serialize message: %v\n", err)
		return err
	}

	err = client.Channel.Publish(
		client.Exchange,
		message.GetRoutingKey(),
		false,
		false,
		amqp.Publishing{
			ContentType: "application/json",
			Body:        body,
		},
	)

	if err != nil {
		log.Printf("Failed to publish message: %v\n", err)
		return err
	}

	log.Println("Message sent successfully.")
	return nil
}

// Consume - отримує повідомлення і передає їх в хендлер
func (client *RabbitMQClient) Consume(handler func(Message)) error {
	log.Println("Starting to consume messages...")

	if client.Connection == nil {
		log.Println("Connection does not exist.")
		return errors.New("connection does not exist")
	}

	if client.Channel == nil {
		log.Println("Channel does not exist.")
		return errors.New("channel does not exist")
	}

	// Отримання повідомлень з черги
	messages, err := client.Channel.Consume(
		client.Queue, // queue name
		"",           // consumer tag
		true,         // auto-ack
		false,        // exclusive
		false,        // no-local
		false,        // no-wait
		nil,          // arguments
	)
	if err != nil {
		log.Printf("Failed to start consuming messages: %v\n", err)
		return err
	}

	// Обробка повідомлень
	go func() {
		for d := range messages {
			log.Println("Received a message")

			var msg Message
			if err := client.Serializer.Unmarshal(d.Body, &msg); err != nil {
				log.Printf("Failed to deserialize message: %v\n", err)
				continue
			}

			handler(msg)
		}
	}()

	log.Println("Consumer started successfully.")
	return nil
}

// Close - закриває канал і з'єднання
func (client *RabbitMQClient) Close() error {
	log.Println("Closing RabbitMQ connection...")

	if err := client.Channel.Close(); err != nil {
		log.Printf("Failed to close channel: %v\n", err)
		return err
	}

	if err := client.Connection.Close(); err != nil {
		log.Printf("Failed to close connection: %v\n", err)
		return err
	}

	log.Println("RabbitMQ connection closed successfully.")
	return nil
}
