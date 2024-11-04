package kafka

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/IBM/sarama"
)

// Producer - структура для работы с Kafka
type Producer struct {
	producer sarama.SyncProducer
}

// NewProducer - создание нового экземпляра Producer
func NewProducer(brokers []string) (*Producer, error) {
	producer, err := sarama.NewSyncProducer(brokers, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create producer: %w", err)
	}
	return &Producer{
		producer: producer,
	}, nil
}

// SendMessage - отправка сообщения в Kafka
func (p *Producer) SendMessage(topic string, key string, value []byte) error {
	msg := &sarama.ProducerMessage{
		Topic: topic,
		Key:   sarama.StringEncoder(key),
		Value: sarama.ByteEncoder(value),
	}

	_, _, err := p.producer.SendMessage(msg)
	if err != nil {
		return fmt.Errorf("failed to send message: %w", err)
	}
	return nil
}

// Close - закрытие Producer
func (p *Producer) Close() error {
	return p.producer.Close()
}

// Consumer - структура для работы с Kafka
type Consumer struct {
	consumer sarama.Consumer
}

// NewConsumer - создание нового экземпляра Consumer
func NewConsumer(brokers []string) (*Consumer, error) {
	consumer, err := sarama.NewConsumer(brokers, nil)
	if err != nil {
		return nil, fmt.Errorf("failed to create consumer: %w", err)
	}
	return &Consumer{
		consumer: consumer,
	}, nil
}

// Consume - потребление сообщений из Kafka
func (c *Consumer) Consume(topic string, partition int32, offset int64) (<-chan *sarama.ConsumerMessage, error) {
	partConsumer, err := c.consumer.ConsumePartition(topic, partition, offset)
	if err != nil {
		return nil, fmt.Errorf("failed to consume partition: %w", err)
	}
	return partConsumer.Messages(), nil
}

// Close - закрытие Consumer
func (c *Consumer) Close() error {
	return c.consumer.Close()
}

// Config - структура для хранения конфигурации
type Config struct {
	KafkaBrokers             []string `json:"kafka_brokers"`
	TopicResponse            string   `json:"topic_response"`
	TopicReceived            string   `json:"topic_received"`
	TopicReceivedAddComments string   `json:"topic_received_add_comments"`
}

// readConfig - функция для чтения конфигурации из файла
func ReadConfig(filePath string) (*Config, error) {
	// Чтение содержимого файла
	data, err := ioutil.ReadFile(filePath)
	if err != nil {
		return nil, fmt.Errorf("failed to read config file: %w", err)
	}

	// Декодирование JSON данных
	var config Config
	err = json.Unmarshal(data, &config)
	if err != nil {
		return nil, fmt.Errorf("failed to unmarshal config data: %w", err)
	}

	return &config, nil
}
