package kafka

import (
	"fmt"
	"io/ioutil"
	"os"

	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
)

// Mock Producer
type MockProducer struct {
	mock.Mock
}

func (m *MockProducer) SendMessage(topic string, key string, value []byte) error {
	args := m.Called(topic, key, value)
	return args.Error(0)
}

func (m *MockProducer) Close() error {
	return m.Called().Error(0)
}

// Test Producer
func TestSendMessage(t *testing.T) {
	mockProd := new(MockProducer)

	// Настройка ожиданий
	mockProd.On("SendMessage", "test_topic", "test_key", []byte("test_value")).Return(nil)

	// Выполнение теста
	err := mockProd.SendMessage("test_topic", "test_key", []byte("test_value"))

	// Проверка
	assert.NoError(t, err)
	mockProd.AssertExpectations(t) // Проверка выполнения всех ожидаемых вызовов
}

func TestSendMessageError(t *testing.T) {
	mockProd := new(MockProducer)

	// Настройка ожиданий на ошибку
	mockProd.On("SendMessage", "test_topic", "test_key", []byte("test_value")).Return(fmt.Errorf("some error"))

	// Выполнение теста
	err := mockProd.SendMessage("test_topic", "test_key", []byte("test_value"))

	// Проверка
	assert.Error(t, err)
	assert.EqualError(t, err, "some error")
	mockProd.AssertExpectations(t) // Проверка выполнения всех ожидаемых вызовов
}

func TestProducerClose(t *testing.T) {
	mockProd := new(MockProducer)

	// Настройка ожиданий
	mockProd.On("Close").Return(nil)

	// Выполнение теста
	err := mockProd.Close()

	// Проверка
	assert.NoError(t, err)
	mockProd.AssertExpectations(t) // Проверка выполнения всех ожидаемых вызовов
}

// Config
func TestReadConfig_Success(t *testing.T) {
	// Создаем временный файл с корректным содержимым
	tmpFile, err := ioutil.TempFile("", "config.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) // Удаляем файл после теста

	// Записываем корректные данные в файл
	configData := `{
        "kafka_brokers": ["localhost:9092"],
        "topic_response_news": "response_news",
        "topic_received_news": "received_news"
    }`
	if _, err := tmpFile.Write([]byte(configData)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Читаем конфигурацию
	config, err := ReadConfig(tmpFile.Name())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}

	// Проверяем корректность значений
	if len(config.KafkaBrokers) != 1 || config.KafkaBrokers[0] != "localhost:9092" {
		t.Fatalf("unexpected kafka_brokers: %v", config.KafkaBrokers)
	}
	if config.TopicResponseNews != "response_news" {
		t.Fatalf("unexpected topic_response_news: %s", config.TopicResponseNews)
	}
	if config.TopicReceivedNews != "received_news" {
		t.Fatalf("unexpected topic_received_news: %s", config.TopicReceivedNews)
	}
}

func TestReadConfig_FileNotFound(t *testing.T) {
	// Попробуем прочитать несуществующий файл
	_, err := ReadConfig("non_existing_file.json")
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}

func TestReadConfig_InvalidJSON(t *testing.T) {
	// Создаем временный файл с некорректным JSON
	tmpFile, err := ioutil.TempFile("", "invalid_config.json")
	if err != nil {
		t.Fatalf("failed to create temp file: %v", err)
	}
	defer os.Remove(tmpFile.Name()) // Удаляем файл после теста

	// Записываем некорректные данные в файл
	if _, err := tmpFile.Write([]byte(`{ "kafka_brokers": ["localhost:9092"], "topic_response_news": "response_news" "topic_received_news": "received_news" }`)); err != nil {
		t.Fatalf("failed to write to temp file: %v", err)
	}
	tmpFile.Close()

	// Читаем конфигурацию
	_, err = ReadConfig(tmpFile.Name())
	if err == nil {
		t.Fatalf("expected error, got none")
	}
}
