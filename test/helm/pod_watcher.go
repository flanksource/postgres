package helm

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"strings"
	"sync"
	"time"

	commonsLogger "github.com/flanksource/commons/logger"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/watch"
	"k8s.io/client-go/kubernetes"
	"k8s.io/client-go/tools/clientcmd"
)

// Terminal failure reasons that should cause immediate test failure
var terminalFailureReasons = map[string]bool{
	"Init:CrashLoopBackOff":        true,
	"CrashLoopBackOff":             true,
	"ImagePullBackOff":             true,
	"ErrImagePull":                 true,
	"InvalidImageName":             true,
	"CreateContainerConfigError":  true,
	"CreateContainerError":         true,
	"RunContainerError":            true,
	"PreStartHookError":            true,
	"PostStartHookError":           true,
}

// PodWatcher watches pods in a namespace and streams their logs
type PodWatcher struct {
	namespace   string
	kubeconfig  string
	clientset   *kubernetes.Clientset
	ctx         context.Context
	cancel      context.CancelFunc
	logger      commonsLogger.Logger
	seenPods    map[string]bool
	logStreams  map[string]context.CancelFunc
	failureChan chan error
	mu          sync.Mutex
	wg          sync.WaitGroup
}

// NewPodWatcher creates a new pod watcher for the given namespace
func NewPodWatcher(namespace, kubeconfig string, logger commonsLogger.Logger) (*PodWatcher, error) {
	// Build kubernetes client config
	config, err := clientcmd.BuildConfigFromFlags("", kubeconfig)
	if err != nil {
		return nil, fmt.Errorf("failed to build kubeconfig: %w", err)
	}

	// Create clientset
	clientset, err := kubernetes.NewForConfig(config)
	if err != nil {
		return nil, fmt.Errorf("failed to create kubernetes client: %w", err)
	}

	ctx, cancel := context.WithCancel(context.Background())

	return &PodWatcher{
		namespace:   namespace,
		kubeconfig:  kubeconfig,
		clientset:   clientset,
		ctx:         ctx,
		cancel:      cancel,
		logger:      logger,
		seenPods:    make(map[string]bool),
		logStreams:  make(map[string]context.CancelFunc),
		failureChan: make(chan error, 10),
	}, nil
}

// Start begins watching pods in the namespace
func (pw *PodWatcher) Start() error {
	pw.logger.Infof("Starting pod watcher for namespace: %s", pw.namespace)

	pw.wg.Add(1)
	go pw.watchPods()

	return nil
}

// Stop stops the pod watcher and all log streams
func (pw *PodWatcher) Stop() {
	pw.logger.Infof("Stopping pod watcher")
	pw.cancel()

	// Cancel all log streams
	pw.mu.Lock()
	for _, cancelFunc := range pw.logStreams {
		cancelFunc()
	}
	pw.mu.Unlock()

	pw.wg.Wait()
	close(pw.failureChan)
}

// Wait blocks until a failure is detected or context is cancelled
func (pw *PodWatcher) Wait() error {
	select {
	case err := <-pw.failureChan:
		return err
	case <-pw.ctx.Done():
		return nil
	}
}

// FailureChan returns the channel that receives failure errors
func (pw *PodWatcher) FailureChan() <-chan error {
	return pw.failureChan
}

// watchPods watches for pod events in the namespace
func (pw *PodWatcher) watchPods() {
	defer pw.wg.Done()

	for {
		select {
		case <-pw.ctx.Done():
			return
		default:
		}

		// Create watcher
		watcher, err := pw.clientset.CoreV1().Pods(pw.namespace).Watch(pw.ctx, metav1.ListOptions{})
		if err != nil {
			pw.logger.Errorf("Failed to create pod watcher: %v", err)
			time.Sleep(5 * time.Second)
			continue
		}

		// Process events
		for event := range watcher.ResultChan() {
			if event.Type == watch.Error {
				// Check if context was cancelled - this is expected during shutdown
				select {
				case <-pw.ctx.Done():
					watcher.Stop()
					return
				default:
					// Only log if it's not a context cancellation error
					errMsg := fmt.Sprintf("%v", event.Object)
					if !strings.Contains(errMsg, "context canceled") {
						pw.logger.Errorf("Watch error: %v", event.Object)
					}
				}
				break
			}

			pod, ok := event.Object.(*corev1.Pod)
			if !ok {
				continue
			}

			switch event.Type {
			case watch.Added, watch.Modified:
				pw.handlePod(pod)
			}
		}

		// Watcher closed, check if we should restart or stop
		watcher.Stop()
		select {
		case <-pw.ctx.Done():
			return
		default:
			time.Sleep(1 * time.Second)
		}
	}
}

// handlePod processes a pod event
func (pw *PodWatcher) handlePod(pod *corev1.Pod) {
	pw.mu.Lock()
	podKey := pod.Name
	alreadySeen := pw.seenPods[podKey]
	pw.mu.Unlock()

	// Check for terminal failures
	if err := pw.checkPodStatus(pod); err != nil {
		select {
		case pw.failureChan <- err:
		default:
		}
		return
	}

	// Start log streaming for new pods
	if !alreadySeen {
		pw.mu.Lock()
		pw.seenPods[podKey] = true
		pw.mu.Unlock()

		pw.logger.Infof("New pod detected: %s (phase: %s)", pod.Name, pod.Status.Phase)

		// Start streaming logs for init containers
		for _, container := range pod.Spec.InitContainers {
			pw.wg.Add(1)
			go pw.streamContainerLogs(pod.Name, container.Name, true)
		}

		// Start streaming logs for regular containers
		for _, container := range pod.Spec.Containers {
			pw.wg.Add(1)
			go pw.streamContainerLogs(pod.Name, container.Name, false)
		}
	}
}

// checkPodStatus checks if the pod is in a terminal failure state
func (pw *PodWatcher) checkPodStatus(pod *corev1.Pod) error {
	// Check init container statuses
	for _, status := range pod.Status.InitContainerStatuses {
		if status.State.Waiting != nil {
			reason := status.State.Waiting.Reason
			if terminalFailureReasons[reason] {
				return fmt.Errorf("pod %s init container %s failed: %s - %s",
					pod.Name, status.Name, reason, status.State.Waiting.Message)
			}
		}
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			if status.RestartCount > 3 {
				return fmt.Errorf("pod %s init container %s terminated with exit code %d after %d restarts",
					pod.Name, status.Name, status.State.Terminated.ExitCode, status.RestartCount)
			}
		}
	}

	// Check regular container statuses
	for _, status := range pod.Status.ContainerStatuses {
		if status.State.Waiting != nil {
			reason := status.State.Waiting.Reason
			if terminalFailureReasons[reason] {
				return fmt.Errorf("pod %s container %s failed: %s - %s",
					pod.Name, status.Name, reason, status.State.Waiting.Message)
			}
		}
		if status.State.Terminated != nil && status.State.Terminated.ExitCode != 0 {
			if status.RestartCount > 5 {
				return fmt.Errorf("pod %s container %s terminated with exit code %d after %d restarts",
					pod.Name, status.Name, status.State.Terminated.ExitCode, status.RestartCount)
			}
		}
	}

	// Check pod phase
	if pod.Status.Phase == corev1.PodFailed {
		return fmt.Errorf("pod %s failed: %s", pod.Name, pod.Status.Reason)
	}

	return nil
}

// streamContainerLogs streams logs from a container
func (pw *PodWatcher) streamContainerLogs(podName, containerName string, isInitContainer bool) {
	defer pw.wg.Done()

	streamKey := fmt.Sprintf("%s/%s", podName, containerName)
	ctx, cancel := context.WithCancel(pw.ctx)

	pw.mu.Lock()
	pw.logStreams[streamKey] = cancel
	pw.mu.Unlock()

	defer func() {
		pw.mu.Lock()
		delete(pw.logStreams, streamKey)
		pw.mu.Unlock()
	}()

	// Wait for container to be ready and started
	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		pod, err := pw.clientset.CoreV1().Pods(pw.namespace).Get(ctx, podName, metav1.GetOptions{})
		if err != nil {
			time.Sleep(1 * time.Second)
			continue
		}

		// Check if container exists and has started
		var containerReady bool
		if isInitContainer {
			for _, status := range pod.Status.InitContainerStatuses {
				if status.Name == containerName {
					// Only proceed if container is running or terminated
					if status.State.Running != nil || status.State.Terminated != nil {
						containerReady = true
						break
					}
					// Container is still waiting (ContainerCreating, PodInitializing, etc.)
					// This is normal, just wait
				}
			}
		} else {
			for _, status := range pod.Status.ContainerStatuses {
				if status.Name == containerName {
					// Only proceed if container is running or terminated
					if status.State.Running != nil || status.State.Terminated != nil {
						containerReady = true
						break
					}
					// Container is still waiting (ContainerCreating, PodInitializing, etc.)
					// This is normal, just wait
				}
			}
		}

		if containerReady {
			break
		}

		time.Sleep(1 * time.Second)
	}

	// Stream logs
	logOptions := &corev1.PodLogOptions{
		Container: containerName,
		Follow:    true,
	}

	req := pw.clientset.CoreV1().Pods(pw.namespace).GetLogs(podName, logOptions)
	stream, err := req.Stream(ctx)
	if err != nil {
		// Only log error if it's not a "container not yet started" error
		if !strings.Contains(err.Error(), "waiting to start") && !strings.Contains(err.Error(), "PodInitializing") {
			pw.logger.Errorf("Failed to stream logs for %s/%s: %v", podName, containerName, err)
		}
		return
	}
	defer stream.Close()

	prefix := fmt.Sprintf("[%s/%s]", podName, containerName)
	reader := bufio.NewReader(stream)

	for {
		select {
		case <-ctx.Done():
			return
		default:
		}

		line, err := reader.ReadString('\n')
		if err != nil {
			if err != io.EOF {
				pw.logger.Errorf("%s Log stream error: %v", prefix, err)
			}
			return
		}

		pw.logger.Infof("%s %s", prefix, line)
	}
}
