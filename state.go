package main

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
)

func reportState(cfg *Config, state string) error {
	body := fmt.Sprintf(`{"token":%q,"state":%q}`, cfg.Token, state)
	resp, err := http.Post(cfg.ApiURL+"/api/agent", "application/json", strings.NewReader(body))
	if err != nil {
		return err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return fmt.Errorf("reportState: status %d — %s", resp.StatusCode, b)
	}
	return nil
}

func readCommand(cfg *Config) (string, error) {
	resp, err := http.Get(cfg.ApiURL + "/api/agent?token=" + cfg.Token)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode >= 400 {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("readCommand: status %d — %s", resp.StatusCode, b)
	}
	var result struct {
		PendingCommand *string `json:"pendingCommand"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return "", err
	}
	if result.PendingCommand == nil {
		return "", nil
	}
	return *result.PendingCommand, nil
}
