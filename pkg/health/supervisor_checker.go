package health

import (
	"fmt"
	"os/exec"
	"strings"
	"time"
)

// SupervisorChecker monitors supervisord managed processes
type SupervisorChecker struct {
	EnabledServices []string // Services that should be running (based on configuration)
}

// NewSupervisorChecker creates a new supervisor health checker
func NewSupervisorChecker(enabledServices []string) *SupervisorChecker {
	return &SupervisorChecker{
		EnabledServices: enabledServices,
	}
}

// Status implements the health.ICheckable interface
func (c *SupervisorChecker) Status() (interface{}, error) {
	status := map[string]interface{}{
		"timestamp":        time.Now(),
		"supervisor_running": false,
		"processes":        []map[string]interface{}{},
		"enabled_services": c.EnabledServices,
	}

	// Check if supervisorctl is available
	if _, err := exec.LookPath("supervisorctl"); err != nil {
		return status, fmt.Errorf("supervisorctl not available: %w", err)
	}

	// Get supervisord status
	output, err := exec.Command("supervisorctl", "status").CombinedOutput()
	if err != nil {
		return status, fmt.Errorf("supervisorctl status failed: %w, output: %s", err, string(output))
	}

	status["supervisor_running"] = true
	processes := []map[string]interface{}{}
	processStates := make(map[string]string)

	// Parse supervisor output
	lines := strings.Split(string(output), "\n")
	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		parts := strings.Fields(line)
		if len(parts) >= 2 {
			processName := parts[0]
			processStatus := parts[1]
			processStates[processName] = processStatus

			process := map[string]interface{}{
				"name":   processName,
				"status": processStatus,
			}

			if len(parts) > 2 {
				process["details"] = strings.Join(parts[2:], " ")
			}

			processes = append(processes, process)
		}
	}

	status["processes"] = processes

	// Check if all enabled services are running
	var unhealthyServices []string
	for _, service := range c.EnabledServices {
		if state, exists := processStates[service]; exists {
			if !isProcessHealthy(state) {
				unhealthyServices = append(unhealthyServices, fmt.Sprintf("%s (%s)", service, state))
			}
		} else {
			unhealthyServices = append(unhealthyServices, fmt.Sprintf("%s (not found)", service))
		}
	}

	if len(unhealthyServices) > 0 {
		return status, fmt.Errorf("unhealthy services: %s", strings.Join(unhealthyServices, ", "))
	}

	status["status"] = "healthy"
	return status, nil
}

// isProcessHealthy returns true if the process state indicates it's healthy
func isProcessHealthy(state string) bool {
	healthyStates := []string{"RUNNING"}
	for _, healthyState := range healthyStates {
		if state == healthyState {
			return true
		}
	}
	return false
}