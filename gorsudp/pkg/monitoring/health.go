// Package monitoring provides health checking and metrics collection
package monitoring

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"sync"
	"time"
)

// HealthStatus represents the overall health status
type HealthStatus string

const (
	HealthStatusHealthy   HealthStatus = "healthy"
	HealthStatusDegraded  HealthStatus = "degraded"
	HealthStatusUnhealthy HealthStatus = "unhealthy"
)

// ComponentHealth represents the health of a system component
type ComponentHealth struct {
	Name          string                 `json:"name"`
	Status        HealthStatus           `json:"status"`
	Message       string                 `json:"message,omitempty"`
	Details       map[string]interface{} `json:"details,omitempty"`
	LastCheck     time.Time              `json:"last_check"`
	CheckDuration time.Duration          `json:"check_duration"`
}

// HealthCheck represents a health check function
type HealthCheck func(ctx context.Context) ComponentHealth

// HealthChecker manages health checks for the system
type HealthChecker struct {
	checks     map[string]HealthCheck
	results    map[string]ComponentHealth
	mutex      sync.RWMutex
	startTime  time.Time
	httpServer *http.Server
}

// SystemHealth represents the overall system health
type SystemHealth struct {
	Status     HealthStatus               `json:"status"`
	Timestamp  time.Time                  `json:"timestamp"`
	Uptime     time.Duration              `json:"uptime"`
	Components map[string]ComponentHealth `json:"components"`
	Metrics    SystemMetrics              `json:"metrics"`
}

// SystemMetrics contains system performance metrics
type SystemMetrics struct {
	Memory     MemoryMetrics `json:"memory"`
	CPU        CPUMetrics    `json:"cpu"`
	Goroutines int           `json:"goroutines"`
	CGOCalls   int64         `json:"cgo_calls"`
}

// MemoryMetrics contains memory-related metrics
type MemoryMetrics struct {
	Alloc      uint64  `json:"alloc_bytes"`
	TotalAlloc uint64  `json:"total_alloc_bytes"`
	Sys        uint64  `json:"sys_bytes"`
	NumGC      uint32  `json:"num_gc"`
	HeapInuse  uint64  `json:"heap_inuse_bytes"`
	HeapIdle   uint64  `json:"heap_idle_bytes"`
	AllocRate  float64 `json:"alloc_rate_bytes_per_sec"`
}

// CPUMetrics contains CPU-related metrics
type CPUMetrics struct {
	UsagePercent float64 `json:"usage_percent"`
	NumCPU       int     `json:"num_cpu"`
}

// NewHealthChecker creates a new health checker
func NewHealthChecker() *HealthChecker {
	return &HealthChecker{
		checks:    make(map[string]HealthCheck),
		results:   make(map[string]ComponentHealth),
		startTime: time.Now(),
	}
}

// RegisterCheck registers a health check for a component
func (hc *HealthChecker) RegisterCheck(name string, check HealthCheck) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	hc.checks[name] = check
	log.Printf("Registered health check: %s", name)
}

// RemoveCheck removes a health check
func (hc *HealthChecker) RemoveCheck(name string) {
	hc.mutex.Lock()
	defer hc.mutex.Unlock()

	delete(hc.checks, name)
	delete(hc.results, name)
	log.Printf("Removed health check: %s", name)
}

// RunChecks executes all registered health checks
func (hc *HealthChecker) RunChecks(ctx context.Context) {
	hc.mutex.Lock()
	checks := make(map[string]HealthCheck)
	for name, check := range hc.checks {
		checks[name] = check
	}
	hc.mutex.Unlock()

	// Run checks concurrently
	var wg sync.WaitGroup
	results := make(chan ComponentHealth, len(checks))

	for name, check := range checks {
		wg.Add(1)
		go func(name string, check HealthCheck) {
			defer wg.Done()

			start := time.Now()
			result := check(ctx)
			result.LastCheck = start
			result.CheckDuration = time.Since(start)
			result.Name = name

			results <- result
		}(name, check)
	}

	// Close results channel when all checks complete
	go func() {
		wg.Wait()
		close(results)
	}()

	// Collect results
	hc.mutex.Lock()
	for result := range results {
		hc.results[result.Name] = result
	}
	hc.mutex.Unlock()
}

// GetHealth returns the current system health status
func (hc *HealthChecker) GetHealth() SystemHealth {
	hc.mutex.RLock()
	defer hc.mutex.RUnlock()

	components := make(map[string]ComponentHealth)
	overallStatus := HealthStatusHealthy

	for name, result := range hc.results {
		components[name] = result

		// Determine overall status
		switch result.Status {
		case HealthStatusUnhealthy:
			overallStatus = HealthStatusUnhealthy
		case HealthStatusDegraded:
			if overallStatus == HealthStatusHealthy {
				overallStatus = HealthStatusDegraded
			}
		}
	}

	return SystemHealth{
		Status:     overallStatus,
		Timestamp:  time.Now(),
		Uptime:     time.Since(hc.startTime),
		Components: components,
		Metrics:    collectSystemMetrics(),
	}
}

// StartHTTPServer starts the health check HTTP server
func (hc *HealthChecker) StartHTTPServer(addr string) error {
	mux := http.NewServeMux()

	// Health endpoint
	mux.HandleFunc("/health", hc.handleHealth)
	mux.HandleFunc("/health/live", hc.handleLiveness)
	mux.HandleFunc("/health/ready", hc.handleReadiness)
	mux.HandleFunc("/metrics", hc.handleMetrics)

	hc.httpServer = &http.Server{
		Addr:    addr,
		Handler: mux,
	}

	log.Printf("Starting health check server on %s", addr)
	return hc.httpServer.ListenAndServe()
}

// Stop stops the health checker
func (hc *HealthChecker) Stop() error {
	if hc.httpServer != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return hc.httpServer.Shutdown(ctx)
	}
	return nil
}

// HTTP Handlers

func (hc *HealthChecker) handleHealth(w http.ResponseWriter, r *http.Request) {
	// Run health checks with timeout
	ctx, cancel := context.WithTimeout(r.Context(), 10*time.Second)
	defer cancel()

	hc.RunChecks(ctx)
	health := hc.GetHealth()

	// Set appropriate status code
	statusCode := http.StatusOK
	switch health.Status {
	case HealthStatusDegraded:
		statusCode = http.StatusOK // Still operational
	case HealthStatusUnhealthy:
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(health)
}

func (hc *HealthChecker) handleLiveness(w http.ResponseWriter, r *http.Request) {
	// Simple liveness check - if we can respond, we're alive
	response := map[string]interface{}{
		"status":    "alive",
		"timestamp": time.Now(),
		"uptime":    time.Since(hc.startTime).String(),
	}

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (hc *HealthChecker) handleReadiness(w http.ResponseWriter, r *http.Request) {
	// Readiness check - run critical health checks only
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()

	hc.RunChecks(ctx)
	health := hc.GetHealth()

	// Ready if not unhealthy
	ready := health.Status != HealthStatusUnhealthy

	response := map[string]interface{}{
		"ready":     ready,
		"status":    health.Status,
		"timestamp": time.Now(),
	}

	statusCode := http.StatusOK
	if !ready {
		statusCode = http.StatusServiceUnavailable
	}

	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(statusCode)
	json.NewEncoder(w).Encode(response)
}

func (hc *HealthChecker) handleMetrics(w http.ResponseWriter, r *http.Request) {
	metrics := collectSystemMetrics()

	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(metrics)
}

// collectSystemMetrics collects system performance metrics
func collectSystemMetrics() SystemMetrics {
	var m runtime.MemStats
	runtime.ReadMemStats(&m)

	return SystemMetrics{
		Memory: MemoryMetrics{
			Alloc:      m.Alloc,
			TotalAlloc: m.TotalAlloc,
			Sys:        m.Sys,
			NumGC:      m.NumGC,
			HeapInuse:  m.HeapInuse,
			HeapIdle:   m.HeapIdle,
		},
		CPU: CPUMetrics{
			NumCPU: runtime.NumCPU(),
		},
		Goroutines: runtime.NumGoroutine(),
		CGOCalls:   runtime.NumCgoCall(),
	}
}

// Default Health Checks

// CreateUDPPortCheck creates a health check for UDP port availability
func CreateUDPPortCheck(port int) HealthCheck {
	return func(ctx context.Context) ComponentHealth {
		// This is a simplified check - in production you might want to actually try binding
		if port < 1 || port > 65535 {
			return ComponentHealth{
				Status:  HealthStatusUnhealthy,
				Message: fmt.Sprintf("invalid port: %d", port),
			}
		}

		return ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: fmt.Sprintf("UDP port %d configured", port),
			Details: map[string]interface{}{
				"port": port,
			},
		}
	}
}

// CreateMemoryCheck creates a health check for memory usage
func CreateMemoryCheck(maxMemoryMB uint64) HealthCheck {
	return func(ctx context.Context) ComponentHealth {
		var m runtime.MemStats
		runtime.ReadMemStats(&m)

		allocMB := m.Alloc / 1024 / 1024

		status := HealthStatusHealthy
		message := fmt.Sprintf("Memory usage: %d MB", allocMB)

		if maxMemoryMB > 0 {
			usagePercent := float64(allocMB) / float64(maxMemoryMB) * 100

			if usagePercent > 90 {
				status = HealthStatusUnhealthy
				message = fmt.Sprintf("Memory usage critical: %.1f%% (%d/%d MB)", usagePercent, allocMB, maxMemoryMB)
			} else if usagePercent > 75 {
				status = HealthStatusDegraded
				message = fmt.Sprintf("Memory usage high: %.1f%% (%d/%d MB)", usagePercent, allocMB, maxMemoryMB)
			} else {
				message = fmt.Sprintf("Memory usage normal: %.1f%% (%d/%d MB)", usagePercent, allocMB, maxMemoryMB)
			}
		}

		return ComponentHealth{
			Status:  status,
			Message: message,
			Details: map[string]interface{}{
				"alloc_mb":   allocMB,
				"max_mb":     maxMemoryMB,
				"sys_mb":     m.Sys / 1024 / 1024,
				"num_gc":     m.NumGC,
				"goroutines": runtime.NumGoroutine(),
			},
		}
	}
}

// CreateDiskSpaceCheck creates a health check for disk space
func CreateDiskSpaceCheck(path string, minFreeGB uint64) HealthCheck {
	return func(ctx context.Context) ComponentHealth {
		// This is a simplified implementation
		// In production, you'd want to use syscall or a library to check actual disk space

		// For now, just check if the path exists and is writable
		if _, err := os.Stat(path); os.IsNotExist(err) {
			return ComponentHealth{
				Status:  HealthStatusUnhealthy,
				Message: fmt.Sprintf("Path does not exist: %s", path),
			}
		}

		// Try to create a test file
		testFile := filepath.Join(path, ".health_check")
		if file, err := os.Create(testFile); err != nil {
			return ComponentHealth{
				Status:  HealthStatusUnhealthy,
				Message: fmt.Sprintf("Cannot write to path: %s", path),
			}
		} else {
			file.Close()
			os.Remove(testFile)
		}

		return ComponentHealth{
			Status:  HealthStatusHealthy,
			Message: fmt.Sprintf("Disk path accessible: %s", path),
			Details: map[string]interface{}{
				"path": path,
			},
		}
	}
}
