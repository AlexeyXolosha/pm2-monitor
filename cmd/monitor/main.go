package main

import (
	"context"
	"log"
	"os/signal"
	"syscall"

	"github.com/AlexeyXolosha/pm2-monitor/internal/config"
	"github.com/AlexeyXolosha/pm2-monitor/internal/monitor"
	"github.com/AlexeyXolosha/pm2-monitor/internal/telegram"
)

// main — точка входа. Здесь мы только "склеиваем" компоненты:
// читаем конфиг → создаём telegram-клиент → создаём монитор → запускаем.
// Вся логика живёт в пакетах internal/*, а main остаётся тонким.
func main() {
	// 1. Загружаем конфигурацию. Если что-то не так (нет токена и т.п.) —
	//    log.Fatalf выведет ошибку и завершит программу с кодом 1.
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("Ошибка конфигурации: %v", err)
	}

	// 2. Создаём клиент Telegram.
	tg, err := telegram.New(cfg.TelegramToken, cfg.TelegramChatID)
	if err != nil {
		log.Fatalf("Ошибка Telegram: %v", err)
	}

	// 3. Настраиваем "плавное" завершение (graceful shutdown).
	//    signal.NotifyContext возвращает context, который отменится,
	//    как только придёт SIGINT (Ctrl+C) или SIGTERM (kill / pm2 stop).
	//    Так процесс завершится корректно, а не будет убит на середине проверки.
	ctx, stop := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer stop()

	// 4. Создаём и запускаем монитор. Start блокируется до отмены контекста.
	m := monitor.New(cfg, tg)
	m.Start(ctx)

	log.Println("Приложение завершено")
}
