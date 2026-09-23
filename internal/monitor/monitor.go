package monitor

import (
	"context"
	"fmt"
	"log"
	"monitoringPM2/internal/config"
	"monitoringPM2/internal/pm2"
	"monitoringPM2/internal/telegram"
	"strings"
	"time"
)

type Monitor struct {
	cfg *config.Config
	tg  *telegram.Client
}

func New(cfg *config.Config, tg *telegram.Client) *Monitor {
	return &Monitor{
		cfg: cfg,
		tg:  tg,
	}
}

// Start запускает бесконечный цикл проверок.
// Принимает context: когда его отменят (например, по Ctrl+C),
// цикл аккуратно завершится, а не оборвётся на полуслове.
func (m *Monitor) Start(ctx context.Context) {
	log.Printf("Мониторинг запущен. Интервал: %s", m.cfg.CheckInterval)

	// Ticker раз в CheckInterval кладёт время в канал ticker.C.
	ticker := time.NewTicker(m.cfg.CheckInterval)
	defer ticker.Stop()

	// Первую проверку делаем сразу, не дожидаясь первого "тика".
	m.check()

	for {
		// select ждёт, что произойдёт раньше:
		//   - отмена контекста (сигнал завершения) → выходим;
		//   - очередной тик таймера → делаем проверку.
		select {
		case <-ctx.Done():
			log.Println("Мониторинг остановлен")
			return
		case <-ticker.C:
			m.check()
		}
	}
}

func (m *Monitor) check() {
	processes, err := pm2.GetProcesses()
	if err != nil {
		log.Printf("Ошибка получения процессов: %v", err)
		return
	}

	var reportBuilder strings.Builder
	reportBuilder.WriteString("📊 <b>Отчет PM2</b>\n\n")

	hasAlerts := false
	var alertsBuilder strings.Builder
	alertsBuilder.WriteString("🚨 <b>ВНИМАНИЕ: ПРОБЛЕМЫ!</b>\n\n")

	for _, p := range processes {
		// Конвертируем байты в мегабайты
		memMB := p.Monit.Memory / 1024 / 1024

		// 1. Формируем строку для общего отчета
		statusIcon := "🟢"
		if p.PM2Env.Status != "online" {
			statusIcon = "🔴"
		}

		reportBuilder.WriteString(fmt.Sprintf("%s <b>%s</b>\n", statusIcon, p.Name))
		reportBuilder.WriteString(fmt.Sprintf("├ Статус: %s\n", p.PM2Env.Status))
		reportBuilder.WriteString(fmt.Sprintf("├ CPU: %.1f%%\n", p.Monit.CPU))
		reportBuilder.WriteString(fmt.Sprintf("├ RAM: %d MB\n", memMB))
		reportBuilder.WriteString(fmt.Sprintf("└ Рестарты: %d\n\n", p.PM2Env.RestartTime))

		// 2. Проверяем триггеры для алертов
		if p.PM2Env.Status != "online" {
			hasAlerts = true
			alertsBuilder.WriteString(fmt.Sprintf("Процесс <b>%s</b> упал! Статус: %s\n", p.Name, p.PM2Env.Status))
		}

		if memMB > m.cfg.MemoryLimitMB {
			hasAlerts = true
			alertsBuilder.WriteString(fmt.Sprintf("Процесс <b>%s</b> жрет память: %d MB (лимит %d MB)\n", p.Name, memMB, m.cfg.MemoryLimitMB))
		}
	}

	if hasAlerts {
		_ = m.tg.SendMessage(alertsBuilder.String())
	} else {
		// Если проблем нет, отправляем обычный отчет (опционально, можно убрать, если нужен только алерт)
		// Для начала оставим, чтобы видеть, что бот работает
		_ = m.tg.SendMessage(reportBuilder.String())
	}
}
