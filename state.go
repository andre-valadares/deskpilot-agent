package main

import (
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"strings"
	"time"
)

var httpClient = &http.Client{Timeout: 15 * time.Second}

func reportState(cfg *Config, state string) error {
	url := cfg.ApiURL + "/api/agent"
	log.Printf("reportState: POST %s state=%s", url, state)
	body := fmt.Sprintf(`{"token":%q,"state":%q}`, cfg.Token, state)
	resp, err := httpClient.Post(url, "application/json", strings.NewReader(body))
	if err != nil {
		return fmt.Errorf("reportState: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return fmt.Errorf("reportState: status %d — %s", resp.StatusCode, b)
	}
	log.Printf("reportState: resposta %d", resp.StatusCode)
	return nil
}

func readCommand(cfg *Config) (string, error) {
	url := cfg.ApiURL + "/api/agent?token=" + cfg.Token
	log.Printf("readCommand: GET %s", url)
	resp, err := httpClient.Get(url)
	if err != nil {
		return "", fmt.Errorf("readCommand: %w", err)
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	if resp.StatusCode >= 400 {
		return "", fmt.Errorf("readCommand: status %d — %s", resp.StatusCode, b)
	}
	log.Printf("readCommand: resposta %d body=%s", resp.StatusCode, strings.TrimSpace(string(b)))
	var result struct {
		PendingCommand *string `json:"pendingCommand"`
	}
	if err := json.Unmarshal(b, &result); err != nil {
		return "", fmt.Errorf("readCommand: decode: %w", err)
	}
	if result.PendingCommand == nil {
		return "", nil
	}
	return *result.PendingCommand, nil
}
