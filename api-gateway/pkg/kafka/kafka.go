package kafka

import (
	"encoding/json"
	"fmt"
	"io/ioutil"

	"github.com/IBM/sarama"
)

// service-news
// Публикация, получаемая из RSS.
type News struct {
	Id         int
	Title      string `json:"title"`
	Content    string `json:"content"`
	PublicTime int64  `json:"public_time"`
	ImageLink  string `json:"image_link"`
	Rubric     string `json:"rubric"`
	Link       string `json:"link"`
	LinkTitle  string `json:"link_title"`
}

// Пагинация.
type Paginate struct {
	PageCurr       int `json:"page_curr"`        //Номер текущей страницы
	PageCount      int `json:"page_count"`       //Количество страниц
	PageCountList  int `json:"page_count_list"`  //Количество новостей на странице
	PageCountTotal int `json:"page_count_total"` //Количество всего новостей
}

// Cтруктура для отправки данных в service
type SendMessServiceNews struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Status    int    `json:"status"`
	TypeQuery string `json:"type_query"`
	Rubric    string `json:"rubric"`
	CountNews int    `json:"count_news"`
	Filter    string `json:"filter"`
	Page      int    `json:"page"`
	IdNews    int    `json:"id_news"`
}

// Cтруктура для получения данных от service
type GetMessServiceNews struct {
	ID        string   `json:"id"`
	Name      string   `json:"name"`
	Status    int      `json:"status"`
	TypeQuery string   `json:"type_query"`
	News      []News   `json:"news"`
	Paginate  Paginate `json:"paginate"`
	IdNews    int      `json:"id_news"`
}

// service-comments
// Комментарий к публикации
type Comment struct {
	Id          int    `json:"id"`
	IdNews      int    `json:"id_news"`
	CommentTime int64  `json:"comment_time"`
	UserName    string `json:"user_name"`
	Content     string `json:"content"`
}

// Cтруктура для передачи данных в service
type SendMessServiceComments struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
	TypeQuery   string `json:"type_query"`
	IdNews      int    `json:"id_news"`
	CommentTime int64  `json:"comment_time"`
	UserName    string `json:"user_name"`
	Content     string `json:"content"`
}

// Cтруктура для получения данных от service
type GetMessServiceComments struct {
	ID        string    `json:"id"`
	Name      string    `json:"name"`
	Status    int       `json:"status"`
	TypeQuery string    `json:"type_query"`
	IdNews    int       `json:"id_news"`
	Comments  []Comment `json:"comments"`
}

type ProducerInterface interface {
	SendMessage(topic string, key string, value []byte) error
	Close() error
}

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

type ConsumerInterface interface {
	Consume(topic string, partition int32, offset int64) (<-chan *sarama.ConsumerMessage, error)
	Close() error
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
	TopicResponseNews        string   `json:"topic_response_news"`
	TopicReceivedNews        string   `json:"topic_received_news"`
	TopicReceivedOneNews     string   `json:"topic_received_one_news"`
	TopicResponseComments    string   `json:"topic_response_comments"`
	TopicReceivedComments    string   `json:"topic_received_comments"`
	TopicReceivedAddComments string   `json:"topic_received_add_comments"`
	TopicResponseCensor      string   `json:"topic_response_censor"`
	TopicReceivedAddCensor   string   `json:"topic_received_add_censor"`
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
