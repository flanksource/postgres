package test

import (
	"bytes"
	"fmt"
	"net"
	"os/exec"
	"strconv"
	"strings"
	"sync"
	"time"
)

// CommandResult holds the result of a command execution
type CommandResult struct {
	Stdout   string
	Stderr   string
	ExitCode int
	Err      error
}

// ANSI color codes for terminal output
const (
	colorReset  = "\033[0m"
	colorGray   = "\033[90m"
	colorRed    = "\033[91m"
	colorYellow = "\033[93m"
	colorBlue   = "\033[94m"
	colorBold   = "\033[1m"
	colorGreen  = "\033[92m"
)

// CommandRunner provides command execution with optional colored output
type CommandRunner struct {
	ColorOutput bool
}

// NewCommandRunner creates a new CommandRunner
func NewCommandRunner(colorOutput bool) *CommandRunner {
	return &CommandRunner{ColorOutput: colorOutput}
}

// RunCommand executes a command and returns the result
func (c *CommandRunner) RunCommand(name string, args ...string) CommandResult {
	if c.ColorOutput {
		fmt.Printf("%s%s>>> Executing: %s %s%s\n", colorBlue, colorBold, name, strings.Join(args, " "), colorReset)
	}

	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	result := CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}

	if c.ColorOutput {
		if result.Err != nil {
			fmt.Printf("%s%s<<< Command failed with exit code %d%s\n", colorRed, colorBold, result.ExitCode, colorReset)
		} else {
			fmt.Printf("%s<<< Command completed successfully%s\n", colorGray, colorReset)
		}
	}

	return result
}

// RunCommandQuiet executes a command without output streaming
func (c *CommandRunner) RunCommandQuiet(name string, args ...string) CommandResult {
	cmd := exec.Command(name, args...)
	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	exitCode := 0
	if exitErr, ok := err.(*exec.ExitError); ok {
		exitCode = exitErr.ExitCode()
	}

	return CommandResult{
		Stdout:   stdout.String(),
		Stderr:   stderr.String(),
		ExitCode: exitCode,
		Err:      err,
	}
}

// Printf prints a formatted colored message
func (c *CommandRunner) Printf(color, style, format string, args ...interface{}) {
	if c.ColorOutput {
		fmt.Printf("%s%s%s%s\n", color, style, fmt.Sprintf(format, args...), colorReset)
	} else {
		fmt.Printf(format+"\n", args...)
	}
}

// Successf prints a success message
func (c *CommandRunner) Successf(format string, args ...interface{}) {
	c.Printf(colorGreen, colorBold, format, args...)
}

// Errorf prints an error message
func (c *CommandRunner) Errorf(format string, args ...interface{}) {
	c.Printf(colorRed, colorBold, format, args...)
}

// Infof prints an info message
func (c *CommandRunner) Infof(format string, args ...interface{}) {
	c.Printf(colorBlue, colorBold, format, args...)
}

// Statusf prints a status message
func (c *CommandRunner) Statusf(format string, args ...interface{}) {
	c.Printf(colorYellow, colorBold, format, args...)
}

// DockerClient provides Docker operations with colored output
type DockerClient struct {
	runner *CommandRunner
}

// NewDockerClient creates a new DockerClient
func NewDockerClient(colorOutput bool) *DockerClient {
	return &DockerClient{
		runner: NewCommandRunner(colorOutput),
	}
}

// Container represents a running Docker container
type Container struct {
	ID     string
	Name   string
	Image  string
	client *DockerClient
}

func (c *Container) WaitFor(timeout time.Duration) error {
	deadline := time.Now().Add(timeout)

	for time.Now().Before(deadline) {
		result := c.client.runner.RunCommandQuiet("docker", "inspect", "-f", "{{.State.Running}}", c.ID)
		if result.ExitCode != 0 {
			return fmt.Errorf("failed to inspect container: %v", result.Err)
		}

		if strings.TrimSpace(result.Stdout) == "false" {
			return nil // Container has stopped
		}

		time.Sleep(1 * time.Second)
	}

	return fmt.Errorf("container did not complete within %v", timeout)
}

// ContainerOptions provides options for running a container
type ContainerOptions struct {
	Name       string
	Image      string
	Command    []string
	Env        map[string]string
	Ports      map[string]string // host:container
	Volumes    map[string]string // host:container
	Network    string
	Detach     bool
	Remove     bool
	Privileged bool
	WorkingDir string
	User       string
}

// Run creates and starts a new Docker container
func Run(opts ContainerOptions) (*Container, error) {
	client := NewDockerClient(true)

	if opts.Image == "" {
		return nil, fmt.Errorf("image is required")
	}

	args := []string{"run"}

	if opts.Detach {
		args = append(args, "-d")
	}

	if opts.Remove {
		args = append(args, "--rm")
	}

	if opts.Privileged {
		args = append(args, "--privileged")
	}

	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}

	if opts.Network != "" {
		args = append(args, "--network", opts.Network)
	}

	if opts.WorkingDir != "" {
		args = append(args, "-w", opts.WorkingDir)
	}

	if opts.User != "" {
		args = append(args, "-u", opts.User)
	}

	// Add environment variables
	for key, value := range opts.Env {
		args = append(args, "-e", fmt.Sprintf("%s=%s", key, value))
	}

	// Add port mappings
	for hostPort, containerPort := range opts.Ports {
		args = append(args, "-p", fmt.Sprintf("%s:%s", hostPort, containerPort))
	}

	// Add volume mappings
	for hostPath, containerPath := range opts.Volumes {
		args = append(args, "-v", fmt.Sprintf("%s:%s", hostPath, containerPath))
	}

	// Add image
	args = append(args, opts.Image)

	// Add command if specified
	if len(opts.Command) > 0 {
		args = append(args, opts.Command...)
	}

	client.runner.Printf(colorBlue, colorBold, "Starting container: %s", opts.Name)

	result := client.runner.RunCommand("docker", args...)
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to run container: %v", result.Err)
	}

	containerID := strings.TrimSpace(result.Stdout)
	if containerID == "" {
		return nil, fmt.Errorf("no container ID returned")
	}

	container := &Container{
		ID:     containerID,
		Name:   opts.Name,
		Image:  opts.Image,
		client: client,
	}

	return container, nil
}

// Stop gracefully stops the container
func (c *Container) Stop() error {
	c.client.runner.Printf(colorGray, "", "Stopping container: %s", c.getIdentifier())
	// Give container 10 seconds to stop gracefully
	result := c.client.runner.RunCommand("docker", "stop", "-t", "10", c.ID)
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to stop container: %v", result.Err)
	}
	return nil
}

// Delete removes the container
func (c *Container) Delete() error {
	c.client.runner.Printf(colorRed, "", "Deleting container: %s", c.getIdentifier())
	result := c.client.runner.RunCommand("docker", "rm", "-f", c.ID)
	if result.ExitCode != 0 {
		return fmt.Errorf("failed to delete container: %v", result.Err)
	}
	return nil
}

// Logs retrieves container logs
func (c *Container) Logs() (string, error) {
	result := c.client.runner.RunCommandQuiet("docker", "logs", c.ID)
	if result.ExitCode != 0 {
		return "", fmt.Errorf("failed to get logs: %v", result.Err)
	}
	return result.Stdout + result.Stderr, nil
}

// Exec executes a command inside the container
func (c *Container) Exec(command ...string) (string, error) {
	args := []string{"exec", c.ID}
	args = append(args, command...)

	c.client.runner.Printf(colorGray, "", "Executing in container: %s", strings.Join(command, " "))
	result := c.client.runner.RunCommand("docker", args...)
	if result.ExitCode != 0 {
		return result.Stdout + result.Stderr, fmt.Errorf("command failed with exit code %d: %v", result.ExitCode, result.Err)
	}
	return result.Stdout, nil
}

// getIdentifier returns the container name if available, otherwise the ID
func (c *Container) getIdentifier() string {
	if c.Name != "" {
		return c.Name
	}
	return c.ID[:12] // First 12 chars of ID
}

// Volume represents a Docker volume
type Volume struct {
	Name   string
	Driver string
	client *DockerClient
}

// VolumeOptions provides options for creating a volume
type VolumeOptions struct {
	Name   string
	Driver string
	Labels map[string]string
}

// CreateVolume creates a new Docker volume
func CreateVolume(opts VolumeOptions) (*Volume, error) {
	client := NewDockerClient(true)

	args := []string{"volume", "create"}

	if opts.Name != "" {
		args = append(args, "--name", opts.Name)
	}

	if opts.Driver != "" {
		args = append(args, "--driver", opts.Driver)
	}

	// Add labels
	for key, value := range opts.Labels {
		args = append(args, "--label", fmt.Sprintf("%s=%s", key, value))
	}

	client.runner.Printf(colorBlue, colorBold, "Creating volume: %s", opts.Name)

	result := client.runner.RunCommand("docker", args...)
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to create volume: %v", result.Err)
	}

	volumeName := strings.TrimSpace(result.Stdout)
	if volumeName == "" && opts.Name != "" {
		volumeName = opts.Name
	}

	volume := &Volume{
		Name:   volumeName,
		Driver: opts.Driver,
		client: client,
	}

	client.runner.Printf(colorGray, colorBold, "Successfully created volume: %s", volumeName)
	return volume, nil
}

// GetVolume retrieves an existing Docker volume
func GetVolume(name string) (*Volume, error) {
	client := NewDockerClient(true)

	// Check if volume exists
	result := client.runner.RunCommandQuiet("docker", "volume", "inspect", name)
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("volume not found: %s", name)
	}

	return &Volume{
		Name:   name,
		client: client,
	}, nil
}

// Delete removes the volume
func (v *Volume) Delete() error {
	v.client.runner.Printf(colorRed, "", "Deleting volume: %s", v.Name)

	result := v.client.runner.RunCommand("docker", "volume", "rm", v.Name)
	if result.ExitCode != 0 {
		// Try force delete
		v.client.runner.Printf(colorYellow, "", "Regular delete failed, trying force delete...")
		forceResult := v.client.runner.RunCommand("docker", "volume", "rm", "-f", v.Name)
		if forceResult.ExitCode != 0 {
			return fmt.Errorf("failed to delete volume: %v", forceResult.Err)
		}
	}

	v.client.runner.Printf(colorGray, colorBold, "Successfully deleted volume: %s", v.Name)
	return nil
}

// CloneVolume creates a copy of the volume with a new name
func (v *Volume) CloneVolume(newName string) (*Volume, error) {
	v.client.runner.Printf(colorBlue, colorBold, "Cloning volume: %s to %s", v.Name, newName)

	// Create new volume
	newVolume, err := CreateVolume(VolumeOptions{
		Name:   newName,
		Driver: v.Driver,
	})
	if err != nil {
		return nil, fmt.Errorf("failed to create new volume: %w", err)
	}

	// Create containers to copy data
	sourceContainer, err := Run(ContainerOptions{
		Name:    fmt.Sprintf("volume-clone-src-%d", time.Now().Unix()),
		Image:   "alpine:latest",
		Command: []string{"sleep", "30"},
		Volumes: map[string]string{
			v.Name: "/source:ro",
		},
		Remove: true,
		Detach: true,
	})
	if err != nil {
		newVolume.Delete()
		return nil, fmt.Errorf("failed to create source container: %w", err)
	}
	defer sourceContainer.Delete()

	destContainer, err := Run(ContainerOptions{
		Name:    fmt.Sprintf("volume-clone-dest-%d", time.Now().Unix()),
		Image:   "alpine:latest",
		Command: []string{"sleep", "30"},
		Volumes: map[string]string{
			newName: "/dest",
		},
		Remove: true,
		Detach: true,
	})
	if err != nil {
		newVolume.Delete()
		return nil, fmt.Errorf("failed to create destination container: %w", err)
	}
	defer destContainer.Delete()

	// Copy data using tar
	_, err = sourceContainer.Exec("tar", "-czf", "/tmp/data.tar.gz", "-C", "/source", ".")
	if err != nil {
		newVolume.Delete()
		return nil, fmt.Errorf("failed to create tar: %w", err)
	}

	// Create a sync mechanism
	var wg sync.WaitGroup
	wg.Add(1)

	// Stream tar from source to destination
	go func() {
		defer wg.Done()
		cmd := exec.Command("docker", "exec", sourceContainer.ID, "cat", "/tmp/data.tar.gz")
		tarData, err := cmd.Output()
		if err != nil {
			return
		}

		cmd = exec.Command("docker", "exec", "-i", destContainer.ID, "sh", "-c", "cat > /tmp/data.tar.gz")
		cmd.Stdin = bytes.NewReader(tarData)
		cmd.Run()
	}()

	wg.Wait()
	time.Sleep(1 * time.Second)

	// Extract in destination
	_, err = destContainer.Exec("tar", "-xzf", "/tmp/data.tar.gz", "-C", "/dest")
	if err != nil {
		newVolume.Delete()
		return nil, fmt.Errorf("failed to extract tar: %w", err)
	}

	v.client.runner.Printf(colorGray, colorBold, "Successfully cloned volume: %s -> %s", v.Name, newName)
	return newVolume, nil
}

// ListVolumes lists all Docker volumes
func ListVolumes() ([]*Volume, error) {
	client := NewDockerClient(true)

	result := client.runner.RunCommandQuiet("docker", "volume", "ls", "--format", "{{.Name}}")
	if result.ExitCode != 0 {
		return nil, fmt.Errorf("failed to list volumes: %v", result.Err)
	}

	var volumes []*Volume
	lines := strings.Split(strings.TrimSpace(result.Stdout), "\n")

	for _, name := range lines {
		if name == "" {
			continue
		}

		volumes = append(volumes, &Volume{
			Name:   name,
			client: client,
		})
	}

	return volumes, nil
}

// GetRandomPort finds and returns a random available port
func GetRandomPort() (int, error) {
	listener, err := net.Listen("tcp", ":0")
	if err != nil {
		return 0, fmt.Errorf("failed to find available port: %w", err)
	}
	defer listener.Close()

	addr := listener.Addr().(*net.TCPAddr)
	return addr.Port, nil
}

// GetRandomPorts finds and returns multiple random available ports
func GetRandomPorts(count int) ([]int, error) {
	var ports []int
	var listeners []net.Listener

	// Create all listeners first to reserve ports
	for i := 0; i < count; i++ {
		listener, err := net.Listen("tcp", ":0")
		if err != nil {
			// Close any previously opened listeners
			for _, l := range listeners {
				l.Close()
			}
			return nil, fmt.Errorf("failed to find available port %d: %w", i+1, err)
		}
		listeners = append(listeners, listener)

		addr := listener.Addr().(*net.TCPAddr)
		ports = append(ports, addr.Port)
	}

	// Close all listeners
	for _, listener := range listeners {
		listener.Close()
	}

	return ports, nil
}

// WaitForPort waits for a port to become available on the given host
func WaitForPort(host string, port int, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	address := net.JoinHostPort(host, strconv.Itoa(port))

	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", address, time.Second)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(100 * time.Millisecond)
	}

	return fmt.Errorf("port %d on %s did not become available within %v", port, host, timeout)
}
