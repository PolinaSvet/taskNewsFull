package main

import (
	"context"
	"encoding/json"
	"errors"
	"news-kafka/service-comments/pkg/kafka"
	"news-kafka/service-comments/pkg/logger"
	"news-kafka/service-comments/pkg/storage"
	"news-kafka/service-comments/pkg/storage/postgres"
	"os"
	"sync"

	"fmt"
	"log"

	"github.com/IBM/sarama"
)

// Сервер
type server struct {
	db storage.Interface
}

// Cтруктура для передачи данных в api-gateway
type SendMessServiceComments struct {
	ID        string            `json:"id"`
	Name      string            `json:"name"`
	Status    int               `json:"status"`
	TypeQuery string            `json:"type_query"`
	IdNews    int               `json:"id_news"`
	Comments  []storage.Comment `json:"comments"`
}

// Cтруктура для получения данных от api-gateway
type GetMessServiceComments struct {
	ID          string `json:"id"`
	Name        string `json:"name"`
	Status      int    `json:"status"`
	TypeQuery   string `json:"type_query"`
	IdNews      int    `json:"id_news"`
	CommentTime int64  `json:"comment_time"`
	UserName    string `json:"user_name"`
	Content     string `json:"content"`
}

func main() {

	fmt.Println("service-comments:", logger.GetServiceName())
	fmt.Println("service-comments:", logger.GetLocalIP())

	// Создаём объект сервера.
	var srv server

	//==============================================
	//Logger
	//==============================================
	logs, err := logger.NewLogger("logs.json", 50)
	if err != nil {
		fmt.Printf("Error creating logger: %v", err)
	}
	defer logs.Close()

	//==============================================
	//Kafka
	//==============================================
	// Чтение конфигурации (предположим, что конфигурация хранится в файле config.json)
	config, err := kafka.ReadConfig("configKafka.json")
	if err != nil {
		log.Fatalf("Failed to read config: %v", err)
	}

	// Создание Kafka Producer и Consumer
	kafkaProducer, err := kafka.NewProducer(config.KafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to create Kafka producer: %v", err)
	}
	defer kafkaProducer.Close()

	kafkaConsumer, err := kafka.NewConsumer(config.KafkaBrokers)
	if err != nil {
		log.Fatalf("Failed to create Kafka consumer: %v", err)
	}
	defer kafkaConsumer.Close()

	// канал для потребления сообщений
	responseCh, err := kafkaConsumer.Consume(config.TopicResponse, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatalf("Failed to consume partition: %v", err)
	}

	//==============================================
	//PostgreSQL
	//==============================================
	// Реляционная БД PostgreSQL.
	connstr := os.Getenv("COMMENTSDBPG")
	if connstr == "" {
		log.Fatal(errors.New("no connection to pg bd"))
	}
	db_pg, err := postgres.New(connstr)
	if err != nil {
		log.Fatal(err)
	}
	srv.db = db_pg
	defer srv.db.Close()

	errorChannel := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	var wg sync.WaitGroup
	wg.Add(2)

	// обрабатываем данные полученные из kafak
	go readNewsFromDB(ctx, srv.db, kafkaProducer, config, responseCh, errorChannel)
	// выводим ошибки
	go handleErrors(ctx, errorChannel, logs)

	wg.Wait()
	//select {}
}

func readNewsFromDB(ctx context.Context, db storage.Interface, producer *kafka.Producer, config *kafka.Config, responseCh <-chan *sarama.ConsumerMessage, errs chan<- error) {
	for msg := range responseCh {
		select {
		case <-ctx.Done():
			return
		default:

			// Обработка входящего сообщения
			var receivedMessage GetMessServiceComments
			err := json.Unmarshal(msg.Value, &receivedMessage)
			if err != nil {
				errs <- err
			}

			//пишем запрос данных в лог
			var errMsg error = receivedMessage
			errs <- errMsg

			responseMessage := SendMessServiceComments{
				ID:        receivedMessage.ID,
				Name:      logger.GetServiceName(),
				TypeQuery: receivedMessage.TypeQuery,
				Status:    0,
				IdNews:    receivedMessage.IdNews,
				Comments:  nil,
			}

			switch receivedMessage.TypeQuery {
			case "CommentsByIdNews":
				comments, err := db.CommentsByIdNews(receivedMessage.IdNews)
				if err != nil {
					errs <- err
				} else {
					responseMessage.Status = 192
					responseMessage.Comments = comments
				}

				bytesMessage, err := json.Marshal(responseMessage)
				if err != nil {
					errs <- err
				}

				err = producer.SendMessage(config.TopicReceived, responseMessage.ID, bytesMessage)
				if err != nil {
					errs <- err
				}
			case "CommentNew":
				comment := storage.Comment{
					Id:          0,
					IdNews:      receivedMessage.IdNews,
					CommentTime: receivedMessage.CommentTime,
					UserName:    receivedMessage.UserName,
					Content:     receivedMessage.Content,
				}

				_, err := db.CommentNew(comment)
				if err != nil {
					errs <- err
				} else {
					responseMessage.Status = 192
				}

				bytesMessage, err := json.Marshal(responseMessage)
				if err != nil {
					errs <- err
				}

				err = producer.SendMessage(config.TopicReceivedAddComments, responseMessage.ID, bytesMessage)
				if err != nil {
					errs <- err
				}
			}

		}
	}
}

func handleErrors(ctx context.Context, errs <-chan error, logs *logger.Logger) {
	for err := range errs {
		select {
		case <-ctx.Done():
			return
		default:
			logs.LogRequest(logger.GetRequestId(), logger.GetLocalIP(), 500, err.Error())
		}
	}
}

// Метод для реализации интерфейса error
func (g GetMessServiceComments) Error() string {
	jsonData, err := json.Marshal(g)
	if err != nil {
		return "error convert to JSON"
	}
	return string(jsonData)
}
