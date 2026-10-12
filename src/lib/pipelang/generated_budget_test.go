package pipelang

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

// Population reservations are shared by the campaign's two workers. Interrupted
// reservations stay charged, so recovery can lose capacity but never delete proof
// or accidentally grant extra capacity. Standalone callers may omit the budget.
func reserveGeneratedPopulation(root string, bytes int64) error {
	path := os.Getenv("PIPELANG_CACHE_BUDGET_FILE")
	if path == "" {
		return nil
	}
	if !filepath.IsAbs(path) {
		return fmt.Errorf("absolute population budget required")
	}
	unlock, err := lockGeneratedArtifact(filepath.Dir(path), "population-budget")
	if err != nil {
		return err
	}
	defer unlock()
	data, err := os.ReadFile(path)
	if err != nil {
		return err
	}
	var budget struct{ Limit, Reserved int64 }
	if err := json.Unmarshal(data, &budget); err != nil {
		return err
	}
	if bytes < 0 || budget.Reserved < 0 || budget.Reserved > budget.Limit || bytes > budget.Limit-budget.Reserved {
		return fmt.Errorf("generated artifact disk budget exhausted; preserved artifacts remain pinned")
	}
	budget.Reserved += bytes
	data, err = json.Marshal(budget)
	if err != nil {
		return err
	}
	file, err := os.CreateTemp(filepath.Dir(path), ".budget-")
	if err != nil {
		return err
	}
	name := file.Name()
	if _, err = file.Write(data); err == nil {
		err = file.Sync()
	}
	closeErr := file.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	if err := os.Rename(name, path); err != nil {
		return err
	}
	directory, err := os.Open(filepath.Dir(path))
	if err != nil {
		return err
	}
	defer directory.Close()
	return directory.Sync()
}
