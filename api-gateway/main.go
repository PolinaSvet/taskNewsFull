package main

import (
	"context"
	"news-kafka/api-gateway/pkg/api"
	"news-kafka/api-gateway/pkg/kafka"
	"news-kafka/api-gateway/pkg/logger"

	"fmt"
	"log"
	"net/http"

	"github.com/IBM/sarama"
)

// tasknews> go test ./... -coverprofile=coverage.out
// tasknews> go test ./... -v -coverprofile=coverage.out

// http://127.0.0.1:8080/news/{rubric}/{countNews}
// http://127.0.0.1:8080/newsDetailed?id_news=1

//https://localhost:9443
//psql -U postgres -d prgComments
//psql -U postgres -d prgNews
//\dt+

// Сервер
type server struct {
	api *api.API
}

func main() {

	fmt.Println("api-gateway:", logger.GetServiceName())
	fmt.Println("api-gateway:", logger.GetLocalIP())

	//==============================================
	//Logger
	//==============================================
	logs, err := logger.NewLogger("logs.json", 50)
	if err != nil {
		fmt.Printf("Error creating logger: %v", err)
	}
	defer logs.Close()

	errorChannel := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	// выводим ошибки
	go handleErrors(ctx, errorChannel, logs)

	// Создаём объект сервера.
	var srv server

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

	// Запуск гоурутины для потребления сообщений service-news
	responseNewsCh, err := kafkaConsumer.Consume(config.TopicReceivedNews, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatalf("Failed to consume partition News: %v", err)
	}

	// Запуск гоурутины для потребления сообщений service-news
	responseOneNewsCh, err := kafkaConsumer.Consume(config.TopicReceivedOneNews, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatalf("Failed to consume partition News: %v", err)
	}

	// Запуск гоурутины для потребления сообщений service-comments
	responseCommentsCh, err := kafkaConsumer.Consume(config.TopicReceivedComments, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatalf("Failed to consume partition Comments: %v", err)
	}

	// Запуск гоурутины для потребления сообщений service-comments
	responseAddCommentsCh, err := kafkaConsumer.Consume(config.TopicReceivedAddComments, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatalf("Failed to consume partition Comments: %v", err)
	}

	// Запуск гоурутины для потребления сообщений service-censor
	responseCensorCh, err := kafkaConsumer.Consume(config.TopicReceivedAddCensor, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatalf("Failed to consume partition Censor: %v", err)
	}

	apiChannels := api.ApiChannels{
		ResponseNewsCh:        responseNewsCh,
		ResponseOneNewsCh:     responseOneNewsCh,
		ResponseCommentsCh:    responseCommentsCh,
		ResponseAddCommentsCh: responseAddCommentsCh,
		ResponseCensorCh:      responseCensorCh,
		ErrorChannel:          errorChannel,
	}

	srv.api = api.New(kafkaProducer, kafkaConsumer, config, apiChannels)

	fmt.Println("Запуск веб-сервера на http://127.0.0.1:8080 ...")
	http.ListenAndServe(":8080", srv.api.Router())
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
