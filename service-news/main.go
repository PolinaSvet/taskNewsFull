package main

import (
	"context"
	"encoding/json"
	"errors"
	"io/ioutil"
	"news-kafka/service-news/pkg/kafka"
	"news-kafka/service-news/pkg/logger"
	"news-kafka/service-news/pkg/rss"
	"news-kafka/service-news/pkg/storage"
	"news-kafka/service-news/pkg/storage/postgres"
	"os"
	"sync"
	"time"

	"fmt"
	"log"

	"github.com/IBM/sarama"
)

type ConfigRSS struct {
	RSS map[string]struct {
		Link  []string `json:"link"`
		Image string   `json:"image"`
	} `json:"rss"`
	Duration int `json:"duration"`
}

// Сервер GoNews
type server struct {
	db storage.Interface
}

// service-news
// Cтруктура для отправки данных -> service-news
type SendMessServiceNews struct {
	ID        string           `json:"id"`
	Name      string           `json:"name"`
	Status    int              `json:"status"`
	TypeQuery string           `json:"type_query"`
	News      []storage.News   `json:"news"`
	Paginate  storage.Paginate `json:"paginate"`
	IdNews    int              `json:"id_news"`
}

// Cтруктура для получения данных <- service-news
type GetMessServiceNews struct {
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

func main() {

	fmt.Println("service-news:", logger.GetServiceName())
	fmt.Println("service-news:", logger.GetLocalIP())

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

	// Запуск гоурутины для потребления сообщений
	responseCh, err := kafkaConsumer.Consume(config.TopicResponse, 0, sarama.OffsetNewest)
	if err != nil {
		log.Fatalf("Failed to consume partition: %v", err)
	}

	//==============================================
	//PostgreSQL
	//==============================================
	// Реляционная БД PostgreSQL.
	connstr := os.Getenv("NEWSDBPG")
	if connstr == "" {
		log.Fatal(errors.New("no connection to pg bd"))
	}
	db_pg, err := postgres.New(connstr)
	if err != nil {
		log.Fatal(err)
	}
	srv.db = db_pg
	defer srv.db.Close()

	//==============================================
	//RSS
	//==============================================
	// чтение и раскодирование файла конфигурации
	data, err := ioutil.ReadFile("./configRSS.json")
	if err != nil {
		log.Fatal(err)
	}
	var configRSS ConfigRSS
	err = json.Unmarshal(data, &configRSS)
	if err != nil {
		log.Fatal(err)
	}

	newsChannel := make(chan []storage.News)
	errorChannel := make(chan error)

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel() // cancel when we are finished consuming integers

	var wg sync.WaitGroup
	wg.Add(4)

	// парсим rss, каждую ссылку в отдельном потоке
	go getNewsFromAllRSS(ctx, configRSS, newsChannel, errorChannel)
	// записываем информацию по каждой ссылке в бд
	go writeNewsToDB(ctx, srv.db, newsChannel, errorChannel)
	// обрабатываем данные полученные из kafak
	go readNewsFromDB(ctx, srv.db, kafkaProducer, config, responseCh, errorChannel)
	// выводим ошибки
	go handleErrors(ctx, errorChannel, logs)

	wg.Wait()
	//select {}
}

func getNewsFromAllRSS(ctx context.Context, configRSS ConfigRSS, news chan<- []storage.News, errs chan<- error) {
	for rubric, value := range configRSS.RSS {
		for _, link := range value.Link {
			go func(url, rubric, image string) {
				for {
					select {
					case <-ctx.Done(): // context checking
						return // returning not to leak the goroutine
					default:
						newsResp, err := rss.GetNewsFromRss(url, rubric, image)
						if err != nil {
							errs <- err
						} else {
							news <- newsResp
						}

						time.Sleep(time.Minute * time.Duration(configRSS.Duration))
					}
				}
			}(link, rubric, value.Image)
		}
	}
}

func readNewsFromDB(ctx context.Context, db storage.Interface, producer *kafka.Producer, config *kafka.Config, responseCh <-chan *sarama.ConsumerMessage, errs chan<- error) {
	for msg := range responseCh {
		select {
		case <-ctx.Done():
			return
		default:

			// Обработка входящего сообщения
			var receivedMessage GetMessServiceNews
			err := json.Unmarshal(msg.Value, &receivedMessage)
			if err != nil {
				errs <- err
			}

			//пишем запрос данных в лог
			var errMsg error = receivedMessage
			errs <- errMsg

			responseMessage := SendMessServiceNews{
				ID:        receivedMessage.ID,
				Name:      logger.GetServiceName(),
				TypeQuery: receivedMessage.TypeQuery,
				Status:    0,
				News:      nil,
				Paginate:  storage.Paginate{},
				IdNews:    receivedMessage.IdNews,
			}

			switch receivedMessage.TypeQuery {
			case "News":
				// Обработка запроса, например, запрос к БД
				news, paginate, err := db.News(receivedMessage.Rubric, receivedMessage.CountNews, receivedMessage.Filter, receivedMessage.Page)
				if err != nil {
					errs <- err
				} else {
					responseMessage.Status = 192
					responseMessage.News = news
					responseMessage.Paginate = paginate
				}

				bytesMessage, err := json.Marshal(responseMessage)
				if err != nil {
					errs <- err
				}

				err = producer.SendMessage(config.TopicReceived, responseMessage.ID, bytesMessage)
				if err != nil {
					errs <- err
				}

			case "OneNews":
				// Обработка запроса, например, запрос к БД
				var news []storage.News
				newsOne, err := db.NewsOne(receivedMessage.IdNews)
				if err != nil {
					errs <- err
				} else {
					news = append(news, newsOne)
					responseMessage.Status = 192
					responseMessage.News = news
				}

				bytesMessage, err := json.Marshal(responseMessage)
				if err != nil {
					errs <- err
				}

				err = producer.SendMessage(config.TopicReceivedOneNews, responseMessage.ID, bytesMessage)
				if err != nil {
					errs <- err
				}

			}

		}
	}
}

func writeNewsToDB(ctx context.Context, db storage.Interface, news <-chan []storage.News, errs chan<- error) {
	for newsBatch := range news {
		select {
		case <-ctx.Done():
			return
		default:
			err := db.AddNew(newsBatch)
			if err != nil {
				errs <- err
				continue
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
func (g GetMessServiceNews) Error() string {
	jsonData, err := json.Marshal(g)
	if err != nil {
		return "Ошибка при преобразовании в JSON"
	}
	return string(jsonData)
}
