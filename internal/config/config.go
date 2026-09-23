package config

import (
	"fmt"
	"os"
	"strconv"
	"time"

	"github.com/joho/godotenv"
)

// Config — все настройки приложения в одном месте.
// Собираем их из переменных окружения (файл .env), чтобы не хардкодить
// секреты (токен бота) прямо в коде.
type Config struct {
	TelegramToken  string        // токен бота из @BotFather
	TelegramChatID string        // куда слать сообщения
	CheckInterval  time.Duration // как часто опрашивать pm2
	MemoryLimitMB  int64         // порог RAM для алерта, в мегабайтах
}

// Load читает .env (если он есть) и переменные окружения, собирает Config
// и проверяет, что обязательные поля заданы.
//
// Возвращаем error, а не паникуем внутри: пусть main сам решит,
// как реагировать на плохую конфигурацию. Это идиоматично для Go.
func Load() (*Config, error) {
	// godotenv.Load подгружает переменные из файла .env в окружение процесса.
	// Если файла нет — это не ошибка: на сервере переменные могут быть
	// заданы через окружение (например, в конфиге pm2). Поэтому ошибку игнорируем.
	_ = godotenv.Load()

	// CHECK_INTERVAL записан как строка вида "1h", "30s", "5m".
	// time.ParseDuration умеет разбирать такой формат.
	interval, err := time.ParseDuration(os.Getenv("CHECK_INTERVAL"))
	if err != nil {
		// Значение не задано или кривое — берём разумное значение по умолчанию.
		interval = 1 * time.Minute
	}

	// MEMORY_LIMIT_MB — число. strconv.ParseInt превращает строку в int64.
	memLimit, err := strconv.ParseInt(os.Getenv("MEMORY_LIMIT_MB"), 10, 64)
	if err != nil || memLimit <= 0 {
		memLimit = 1024 // по умолчанию 1 ГБ
	}

	cfg := &Config{
		TelegramToken:  os.Getenv("TELEGRAM_BOT_TOKEN"),
		TelegramChatID: os.Getenv("TELEGRAM_CHAT_ID"),
		CheckInterval:  interval,
		MemoryLimitMB:  memLimit,
	}

	// Валидация: без токена и chat_id бот работать не сможет,
	// поэтому лучше упасть сразу с понятной ошибкой, чем ловить её потом.
	if cfg.TelegramToken == "" {
		return nil, fmt.Errorf("не задан TELEGRAM_BOT_TOKEN")
	}
	if cfg.TelegramChatID == "" {
		return nil, fmt.Errorf("не задан TELEGRAM_CHAT_ID")
	}

	return cfg, nil
}
