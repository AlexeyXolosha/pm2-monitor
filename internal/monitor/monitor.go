package monitor

import (
	"context"
	"fmt"
	"github.com/AlexeyXolosha/pm2-monitor/internal/config"
	"github.com/AlexeyXolosha/pm2-monitor/internal/pm2"
	"github.com/AlexeyXolosha/pm2-monitor/internal/telegram"
	"log"
	"strings"
	"time"
)

type Monitor struct {
	cfg *config.Config
	tg  *telegram.Client

	// prev — снимок процессов с прошлой проверки, ключ — pm_id.
	// Именно он позволяет отслеживать ИЗМЕНЕНИЯ (например, рост рестартов),
	// а не только текущее состояние.
	prev map[int]pm2.Process
}

func New(cfg *config.Config, tg *telegram.Client) *Monitor {
	return &Monitor{
		cfg:  cfg,
		tg:   tg,
		prev: make(map[int]pm2.Process),
	}
}

// Start запускает бесконечный цикл проверок.
// Принимает context: когда его отменят (например, по Ctrl+C),
// цикл аккуратно завершится, а не оборвётся на полуслове.
func (m *Monitor) Start(ctx context.Context) {
	log.Printf("Мониторинг запущен. Интервал: %s", m.cfg.CheckInterval)

	ticker := time.NewTicker(m.cfg.CheckInterval)
	defer ticker.Stop()

	m.check()

	for {
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

	// 1. Общий отчёт — отправляем всегда, чтобы видеть «в целом всё ок».
	report := m.buildReport(processes)
	if err := m.tg.SendMessage(report); err != nil {
		log.Printf("Не удалось отправить отчёт: %v", err)
	}

	// 2. Алерты — сравниваем с прошлым состоянием и текущими порогами.
	//    Отправляем отдельным сообщением, только если что-то нашли.
	if alerts := m.buildAlerts(processes); alerts != "" {
		if err := m.tg.SendMessage(alerts); err != nil {
			log.Printf("Не удалось отправить алерт: %v", err)
		}
	}

	// 3. Запоминаем текущее состояние — оно станет «прошлым» на следующей проверке.
	snapshot := make(map[int]pm2.Process, len(processes))
	for _, p := range processes {
		snapshot[p.PMID] = p
	}
	m.prev = snapshot
}

// buildReport формирует красивый отчёт по всем процессам,
// сгруппированный по имени приложения, с суммарной RAM в конце.
func (m *Monitor) buildReport(processes []pm2.Process) string {
	var b strings.Builder
	b.WriteString("🖥 <b>PM2 Monitor</b>\n\n")

	// Группируем процессы по имени, сохраняя порядок их появления.
	// order хранит имена в порядке первой встречи, groups — сами процессы.
	var order []string
	groups := make(map[string][]pm2.Process)
	for _, p := range processes {
		if _, ok := groups[p.Name]; !ok {
			order = append(order, p.Name)
		}
		groups[p.Name] = append(groups[p.Name], p)
	}

	var totalBytes int64
	for _, name := range order {
		b.WriteString(fmt.Sprintf("<b>%s</b>\n\n", name))
		for _, p := range groups[name] {
			icon := "🟢"
			if p.PM2Env.Status != "online" {
				icon = "🔴"
			}
			totalBytes += p.Monit.Memory

			b.WriteString(fmt.Sprintf("%s #%d\n", icon, p.PMID))
			b.WriteString(fmt.Sprintf("RAM: %d MB\n", p.Monit.Memory/1024/1024))
			b.WriteString(fmt.Sprintf("CPU: %g%%\n", p.Monit.CPU))
			b.WriteString(fmt.Sprintf("Restarts: %d\n\n", p.PM2Env.RestartTime))
		}
	}

	b.WriteString("────────────────\n")
	b.WriteString(fmt.Sprintf("<b>Total RAM: %s</b>", humanBytes(totalBytes)))
	return b.String()
}

// buildAlerts возвращает текст алерта, если есть проблемы или изменения,
// иначе — пустую строку.
func (m *Monitor) buildAlerts(processes []pm2.Process) string {
	var alerts []string

	for _, p := range processes {
		// а) Рост числа рестартов по сравнению с прошлой проверкой.
		//    Сравнение возможно только если процесс уже был в прошлом снимке.
		if prev, ok := m.prev[p.PMID]; ok && p.PM2Env.RestartTime > prev.PM2Env.RestartTime {
			alerts = append(alerts, fmt.Sprintf(
				"<b>%s #%d</b>\nБыло рестартов: %d\nСтало: %d\n\nПроцесс был перезапущен.",
				p.Name, p.PMID, prev.PM2Env.RestartTime, p.PM2Env.RestartTime,
			))
		}

		// б) Процесс не в статусе online.
		if p.PM2Env.Status != "online" {
			alerts = append(alerts, fmt.Sprintf(
				"<b>%s #%d</b>\nСтатус: %s\n\nПроцесс упал!",
				p.Name, p.PMID, p.PM2Env.Status,
			))
		}

		// в) Превышение лимита памяти.
		memMB := p.Monit.Memory / 1024 / 1024
		if memMB > m.cfg.MemoryLimitMB {
			alerts = append(alerts, fmt.Sprintf(
				"<b>%s #%d</b>\nRAM: %d MB (лимит %d MB)\n\nПревышен лимит памяти!",
				p.Name, p.PMID, memMB, m.cfg.MemoryLimitMB,
			))
		}
	}

	if len(alerts) == 0 {
		return ""
	}
	return "⚠️ <b>PM2 ALERT</b>\n\n" + strings.Join(alerts, "\n\n")
}

// humanBytes переводит байты в человекочитаемый вид: MB или GB.
func humanBytes(b int64) string {
	const mb = 1024 * 1024
	const gb = 1024 * mb
	if b >= gb {
		return fmt.Sprintf("%.2f GB", float64(b)/float64(gb))
	}
	return fmt.Sprintf("%d MB", b/mb)
}
