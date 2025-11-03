package pgtune

import (
	"bufio"
	"fmt"
	"os"
	"regexp"
	"strings"
	"time"
)

// AutoConfParameter represents a single parameter in postgres.auto.conf
type AutoConfParameter struct {
	Name    string
	Value   string
	Comment string // inline comment if any
}

// AutoConfFile represents the parsed postgres.auto.conf file
type AutoConfFile struct {
	Parameters map[string]*AutoConfParameter
	Comments   []string // standalone comments (not associated with parameters)
}

// ParseAutoConf parses a postgres.auto.conf file
func ParseAutoConf(path string) (*AutoConfFile, error) {
	file, err := os.Open(path)
	if err != nil {
		if os.IsNotExist(err) {
			// File doesn't exist, return empty config
			return &AutoConfFile{
				Parameters: make(map[string]*AutoConfParameter),
				Comments:   []string{},
			}, nil
		}
		return nil, fmt.Errorf("failed to open auto.conf: %w", err)
	}
	defer file.Close()

	autoConf := &AutoConfFile{
		Parameters: make(map[string]*AutoConfParameter),
		Comments:   []string{},
	}

	// Regex to parse parameter lines: param = value # optional comment
	paramRegex := regexp.MustCompile(`^\s*([a-zA-Z_][a-zA-Z0-9_]*)\s*=\s*(.+?)(?:\s*#\s*(.*))?$`)
	commentRegex := regexp.MustCompile(`^\s*#(.*)$`)

	scanner := bufio.NewScanner(file)
	for scanner.Scan() {
		line := scanner.Text()
		trimmed := strings.TrimSpace(line)

		// Empty line
		if trimmed == "" {
			continue
		}

		// Standalone comment
		if matches := commentRegex.FindStringSubmatch(line); len(matches) > 1 {
			autoConf.Comments = append(autoConf.Comments, strings.TrimSpace(matches[1]))
			continue
		}

		// Parameter line
		if matches := paramRegex.FindStringSubmatch(line); len(matches) >= 3 {
			paramName := strings.TrimSpace(matches[1])
			paramValue := strings.TrimSpace(matches[2])
			inlineComment := ""
			if len(matches) > 3 {
				inlineComment = strings.TrimSpace(matches[3])
			}

			autoConf.Parameters[paramName] = &AutoConfParameter{
				Name:    paramName,
				Value:   paramValue,
				Comment: inlineComment,
			}
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("error reading auto.conf: %w", err)
	}

	return autoConf, nil
}

func GetDefaults() map[string]string {
	return map[string]string{
		"archive_mode":                "off",
		"archive_timeout":             "0",
		"lc_messages":                 "C",
		"listen_addresses":            "*",
		"log_autovacuum_min_duration": "10s",
		"log_checkpoints":             "true",
		"log_connections":             "false",
		"log_destination":             "stderr",
		"log_disconnections":          "false",
		"log_line_prefix":             "%m [%p] %q[user=%u,db=%d,app=%a]",
		"log_lock_waits":              "true",
		"log_min_duration_statement":  "10s",
		"log_timezone":                "UTC",
		"timezone":                    "UTC",
		"logging_collector":           "true",
		"password_encryption":         "scram-sha-256",
		"ssl":                         "false",
	}
}

// GetPgTuneManagedParams returns the list of parameters managed by pg_tune
func GetPgTuneManagedParams() []string {
	return []string{
		"max_connections",
		"shared_buffers",
		"effective_cache_size",
		"maintenance_work_mem",
		"work_mem",
		"wal_buffers",
		"min_wal_size",
		"max_wal_size",
		"checkpoint_completion_target",
		"random_page_cost",
		"effective_io_concurrency",
		"default_statistics_target",
		"max_worker_processes",
		"max_parallel_workers",
		"max_parallel_workers_per_gather",
		"max_parallel_maintenance_workers",
		"wal_level",
		"max_wal_senders",
		"huge_pages",
	}
}

// MergeWithTunedParams merges tuned parameters into the auto.conf, updating only pg_tune managed params
func (ac *AutoConfFile) MergeWithTunedParams(params *TunedParameters) {
	managedParams := make(map[string]bool)
	for _, p := range GetPgTuneManagedParams() {
		managedParams[p] = true
	}

	// Update or add pg_tune managed parameters
	ac.updateParam("max_connections", fmt.Sprintf("%d", params.MaxConnections))

	ac.updateParam("shared_buffers", params.SharedBuffers.String())

	ac.updateParam("effective_cache_size", params.EffectiveCacheSize.String())

	ac.updateParam("maintenance_work_mem", params.MaintenanceWorkMem.String())

	ac.updateParam("work_mem", params.WorkMem.String())

	ac.updateParam("wal_buffers", params.WalBuffers.String())

	ac.updateParam("min_wal_size", params.MinWalSize.String())

	ac.updateParam("max_wal_size", params.MaxWalSize.String())

	ac.updateParam("checkpoint_completion_target", fmt.Sprintf("%.2f", params.CheckpointCompletionTarget))
	ac.updateParam("random_page_cost", fmt.Sprintf("%.1f", params.RandomPageCost))

	if params.EffectiveIoConcurrency != nil {
		ac.updateParam("effective_io_concurrency", fmt.Sprintf("%d", *params.EffectiveIoConcurrency))
	}

	ac.updateParam("default_statistics_target", fmt.Sprintf("%d", params.DefaultStatisticsTarget))
	ac.updateParam("max_worker_processes", fmt.Sprintf("%d", params.MaxWorkerProcesses))
	ac.updateParam("max_parallel_workers", fmt.Sprintf("%d", params.MaxParallelWorkers))
	ac.updateParam("max_parallel_workers_per_gather", fmt.Sprintf("%d", params.MaxParallelWorkersPerGather))

	if params.MaxParallelMaintenanceWorkers != nil {
		ac.updateParam("max_parallel_maintenance_workers", fmt.Sprintf("%d", *params.MaxParallelMaintenanceWorkers))
	}

	ac.updateParam("wal_level", fmt.Sprintf("'%s'", params.WalLevel))

	if params.MaxWalSenders != nil {
		ac.updateParam("max_wal_senders", fmt.Sprintf("%d", *params.MaxWalSenders))
	}

	ac.updateParam("huge_pages", fmt.Sprintf("'%s'", params.HugePages))
}

// updateParam updates or adds a parameter
func (ac *AutoConfFile) updateParam(name, value string) {
	if existing, ok := ac.Parameters[name]; ok {
		existing.Value = value
	} else {
		ac.Parameters[name] = &AutoConfParameter{
			Name:  name,
			Value: value,
		}
	}
}

// WriteToFile writes the auto.conf to a file
func (ac *AutoConfFile) WriteToFile(path string) error {
	file, err := os.Create(path)
	if err != nil {
		return fmt.Errorf("failed to create auto.conf: %w", err)
	}
	defer file.Close()

	writer := bufio.NewWriter(file)
	defer writer.Flush()

	// Write header
	writer.WriteString("# Do not edit this file manually!\n")
	writer.WriteString("# It will be overwritten by pg_tune --update\n")
	writer.WriteString(fmt.Sprintf("# Last updated: %s\n", time.Now().Format("2006-01-02 15:04:05")))
	writer.WriteString("#\n\n")

	// Write pg_tune managed parameters first
	writer.WriteString("# -----------------------------\n")
	writer.WriteString("# PG_TUNE MANAGED PARAMETERS\n")
	writer.WriteString("# -----------------------------\n\n")

	managedParams := GetPgTuneManagedParams()
	managedSet := make(map[string]bool)
	for _, p := range managedParams {
		managedSet[p] = true
	}

	// Write managed params in order
	for _, paramName := range managedParams {
		if param, ok := ac.Parameters[paramName]; ok {
			if param.Comment != "" {
				writer.WriteString(fmt.Sprintf("%s = %s  # %s\n", param.Name, param.Value, param.Comment))
			} else {
				writer.WriteString(fmt.Sprintf("%s = %s\n", param.Name, param.Value))
			}
		}
	}

	// Write other parameters
	hasOtherParams := false
	for paramName := range ac.Parameters {
		if !managedSet[paramName] {
			hasOtherParams = true
			break
		}
	}

	if hasOtherParams {
		writer.WriteString("\n# -----------------------------\n")
		writer.WriteString("# OTHER PARAMETERS\n")
		writer.WriteString("# -----------------------------\n\n")

		for paramName, param := range ac.Parameters {
			if !managedSet[paramName] {
				if param.Comment != "" {
					writer.WriteString(fmt.Sprintf("%s = %s  # %s\n", param.Name, param.Value, param.Comment))
				} else {
					writer.WriteString(fmt.Sprintf("%s = %s\n", param.Name, param.Value))
				}
			}
		}
	}

	return nil
}
