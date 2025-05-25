package notify

import (
	"context"
	"fmt"
	"log"
	"sync"
	"time"

	"github.com/google/uuid"
)

// NotificationManager manages notification providers and job queue
type NotificationManager struct {
	config    NotificationConfig
	providers map[string]NotificationProvider
	queue     chan NotificationJob
	results   chan NotificationResult
	workers   []*NotificationWorker
	ctx       context.Context
	cancel    context.CancelFunc
	wg        sync.WaitGroup
	mutex     sync.RWMutex
}

// NotificationWorker processes notification jobs
type NotificationWorker struct {
	id       int
	manager  *NotificationManager
	stopChan chan struct{}
}

// NewNotificationManager creates a new notification manager
func NewNotificationManager(config NotificationConfig) *NotificationManager {
	ctx, cancel := context.WithCancel(context.Background())
	
	return &NotificationManager{
		config:    config,
		providers: make(map[string]NotificationProvider),
		queue:     make(chan NotificationJob, config.QueueSize),
		results:   make(chan NotificationResult, config.QueueSize),
		workers:   make([]*NotificationWorker, 0, config.Workers),
		ctx:       ctx,
		cancel:    cancel,
	}
}

// RegisterProvider registers a notification provider
func (nm *NotificationManager) RegisterProvider(provider NotificationProvider) error {
	nm.mutex.Lock()
	defer nm.mutex.Unlock()
	
	if !provider.IsEnabled() {
		log.Printf("Provider %s is disabled, skipping registration", provider.GetName())
		return nil
	}
	
	nm.providers[provider.GetName()] = provider
	log.Printf("Registered notification provider: %s", provider.GetName())
	return nil
}

// Start starts the notification manager and workers
func (nm *NotificationManager) Start() error {
	if !nm.config.Enabled {
		log.Println("Notification manager is disabled")
		return nil
	}
	
	log.Printf("Starting notification manager with %d workers", nm.config.Workers)
	
	// Start result processor
	nm.wg.Add(1)
	go nm.processResults()
	
	// Start workers
	for i := 0; i < nm.config.Workers; i++ {
		worker := &NotificationWorker{
			id:       i,
			manager:  nm,
			stopChan: make(chan struct{}),
		}
		nm.workers = append(nm.workers, worker)
		
		nm.wg.Add(1)
		go worker.start()
	}
	
	log.Println("Notification manager started successfully")
	return nil
}

// Stop stops the notification manager
func (nm *NotificationManager) Stop() error {
	log.Println("Stopping notification manager...")
	
	// Signal cancellation
	nm.cancel()
	
	// Stop all workers
	for _, worker := range nm.workers {
		close(worker.stopChan)
	}
	
	// Close channels
	close(nm.queue)
	close(nm.results)
	
	// Wait for all goroutines to finish
	nm.wg.Wait()
	
	log.Println("Notification manager stopped")
	return nil
}

// SendAlert sends an earthquake alert notification
func (nm *NotificationManager) SendAlert(alert Alert) error {
	if !nm.config.Enabled {
		return nil
	}
	
	message := nm.formatAlertMessage(alert)
	
	// Send to all enabled providers
	nm.mutex.RLock()
	providers := make([]string, 0, len(nm.providers))
	for name := range nm.providers {
		providers = append(providers, name)
	}
	nm.mutex.RUnlock()
	
	for _, providerName := range providers {
		job := NotificationJob{
			ID:         uuid.New().String(),
			Type:       NotificationAlert,
			Provider:   providerName,
			Message:    message,
			Timestamp:  time.Now(),
			Retry:      0,
			MaxRetries: nm.config.MaxRetries,
			Metadata: map[string]interface{}{
				"alert": alert,
			},
		}
		
		select {
		case nm.queue <- job:
		case <-nm.ctx.Done():
			return fmt.Errorf("notification manager is shutting down")
		default:
			log.Printf("Warning: notification queue full, dropping alert for provider %s", providerName)
		}
	}
	
	return nil
}

// SendReset sends an alert reset notification
func (nm *NotificationManager) SendReset(reset Reset) error {
	if !nm.config.Enabled {
		return nil
	}
	
	message := nm.formatResetMessage(reset)
	
	// Send to all enabled providers
	nm.mutex.RLock()
	providers := make([]string, 0, len(nm.providers))
	for name := range nm.providers {
		providers = append(providers, name)
	}
	nm.mutex.RUnlock()
	
	for _, providerName := range providers {
		job := NotificationJob{
			ID:         uuid.New().String(),
			Type:       NotificationReset,
			Provider:   providerName,
			Message:    message,
			Timestamp:  time.Now(),
			Retry:      0,
			MaxRetries: nm.config.MaxRetries,
			Metadata: map[string]interface{}{
				"reset": reset,
			},
		}
		
		select {
		case nm.queue <- job:
		case <-nm.ctx.Done():
			return fmt.Errorf("notification manager is shutting down")
		default:
			log.Printf("Warning: notification queue full, dropping reset for provider %s", providerName)
		}
	}
	
	return nil
}

// SendCustomMessage sends a custom text message
func (nm *NotificationManager) SendCustomMessage(providerName, message string) error {
	if !nm.config.Enabled {
		return nil
	}
	
	nm.mutex.RLock()
	_, exists := nm.providers[providerName]
	nm.mutex.RUnlock()
	
	if !exists {
		return fmt.Errorf("provider %s not found", providerName)
	}
	
	job := NotificationJob{
		ID:         uuid.New().String(),
		Type:       NotificationText,
		Provider:   providerName,
		Message:    message,
		Timestamp:  time.Now(),
		Retry:      0,
		MaxRetries: nm.config.MaxRetries,
	}
	
	select {
	case nm.queue <- job:
		return nil
	case <-nm.ctx.Done():
		return fmt.Errorf("notification manager is shutting down")
	default:
		return fmt.Errorf("notification queue full")
	}
}

// GetProviders returns list of registered providers
func (nm *NotificationManager) GetProviders() []string {
	nm.mutex.RLock()
	defer nm.mutex.RUnlock()
	
	providers := make([]string, 0, len(nm.providers))
	for name := range nm.providers {
		providers = append(providers, name)
	}
	return providers
}

// formatAlertMessage formats an alert message
func (nm *NotificationManager) formatAlertMessage(alert Alert) string {
	return fmt.Sprintf("🚨 EARTHQUAKE ALERT 🚨\n\n"+
		"Station: %s.%s\n"+
		"Channel: %s\n"+
		"Time: %s\n"+
		"STA/LTA Ratio: %.2f (threshold: %.2f)\n"+
		"Duration: %.1fs\n\n"+
		"Please take safety precautions immediately!",
		alert.Network, alert.Station,
		alert.Channel,
		alert.Timestamp.Format("2006-01-02 15:04:05 UTC"),
		alert.STALTARatio, alert.Threshold,
		alert.Duration)
}

// formatResetMessage formats a reset message
func (nm *NotificationManager) formatResetMessage(reset Reset) string {
	return fmt.Sprintf("✅ Alert Reset\n\n"+
		"Station: %s.%s\n"+
		"Channel: %s\n"+
		"Time: %s\n"+
		"Duration: %.1fs\n\n"+
		"Alert conditions have ended.",
		reset.Network, reset.Station,
		reset.Channel,
		reset.Timestamp.Format("2006-01-02 15:04:05 UTC"),
		reset.Duration)
}

// processResults processes notification results
func (nm *NotificationManager) processResults() {
	defer nm.wg.Done()
	
	for {
		select {
		case result := <-nm.results:
			if result.Success {
				log.Printf("Notification sent successfully: provider=%s, job_id=%s, duration=%v",
					result.Provider, result.JobID, result.Duration)
			} else {
				log.Printf("Notification failed: provider=%s, job_id=%s, error=%s",
					result.Provider, result.JobID, result.Error)
			}
		case <-nm.ctx.Done():
			log.Println("Result processor stopping...")
			return
		}
	}
}

// start starts a notification worker
func (nw *NotificationWorker) start() {
	defer nw.manager.wg.Done()
	
	log.Printf("Notification worker %d started", nw.id)
	
	for {
		select {
		case job, ok := <-nw.manager.queue:
			if !ok {
				log.Printf("Notification worker %d stopping (queue closed)", nw.id)
				return
			}
			nw.processJob(job)
			
		case <-nw.stopChan:
			log.Printf("Notification worker %d stopping (stop signal)", nw.id)
			return
			
		case <-nw.manager.ctx.Done():
			log.Printf("Notification worker %d stopping (context cancelled)", nw.id)
			return
		}
	}
}

// processJob processes a single notification job
func (nw *NotificationWorker) processJob(job NotificationJob) {
	start := time.Now()
	
	// Get provider
	nw.manager.mutex.RLock()
	provider, exists := nw.manager.providers[job.Provider]
	nw.manager.mutex.RUnlock()
	
	if !exists {
		result := NotificationResult{
			JobID:     job.ID,
			Provider:  job.Provider,
			Success:   false,
			Error:     "provider not found",
			Timestamp: time.Now(),
			Duration:  time.Since(start),
		}
		
		select {
		case nw.manager.results <- result:
		case <-nw.manager.ctx.Done():
		}
		return
	}
	
	// Send notification
	var err error
	switch job.Type {
	case NotificationText, NotificationAlert, NotificationReset:
		err = provider.SendText(job.Message)
	case NotificationImage:
		err = provider.SendImage(job.Message, job.ImagePath)
	default:
		err = fmt.Errorf("unsupported notification type: %s", job.Type)
	}
	
	result := NotificationResult{
		JobID:     job.ID,
		Provider:  job.Provider,
		Success:   err == nil,
		Timestamp: time.Now(),
		Duration:  time.Since(start),
	}
	
	if err != nil {
		result.Error = err.Error()
		
		// Retry if possible
		if job.Retry < job.MaxRetries {
			job.Retry++
			
			// Schedule retry with delay
			go func() {
				time.Sleep(nw.manager.config.RetryDelay)
				select {
				case nw.manager.queue <- job:
				case <-nw.manager.ctx.Done():
				}
			}()
			
			log.Printf("Retrying notification: provider=%s, job_id=%s, retry=%d/%d",
				job.Provider, job.ID, job.Retry, job.MaxRetries)
		}
	}
	
	// Send result
	select {
	case nw.manager.results <- result:
	case <-nw.manager.ctx.Done():
	}
}