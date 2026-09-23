package pm2

import (
	"encoding/json"
	"fmt"
	"os/exec"
)

type Process struct {
	Name string `json:"name"`
	PMID int    `json:"pm_id"` // номер процесса в pm2 (#0, #1, ...)

	Monit struct {
		Memory int64   `json:"memory"`
		CPU    float64 `json:"cpu"`
	} `json:"monit"`

	PM2Env struct {
		Status      string `json:"status"`
		RestartTime int    `json:"restart_time"`
	} `json:"pm2_env"`
}

func GetProcesses() ([]Process, error) {
	cmd := exec.Command("pm2", "jlist")

	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("ошибка выполнения pm2 jlist: %w", err)
	}

	var processes []Process
	if err := json.Unmarshal(output, &processes); err != nil {
		return nil, fmt.Errorf("ошибка парсинга JSON: %w", err)
	}

	return processes, nil
}
