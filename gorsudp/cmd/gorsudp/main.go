// Package main provides the main entry point for gorsudp
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/tisayama/gorsudp/internal/alert"
	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/internal/notify"
	"github.com/tisayama/gorsudp/internal/plot"
	"github.com/tisayama/gorsudp/internal/producer"
	"github.com/tisayama/gorsudp/internal/screenshot"
	"github.com/tisayama/gorsudp/internal/writer"
	"github.com/tisayama/gorsudp/pkg/config"
	"github.com/tisayama/gorsudp/pkg/monitoring"
	"github.com/tisayama/gorsudp/pkg/stream"
)

const (
	appName    = "gorsudp"
	appVersion = "0.1.0"
)

func main() {
	// Parse command line flags
	var (
		configFile  = flag.String("config", "", "Configuration file path")
		genConfig   = flag.Bool("generate-config", false, "Generate default configuration file")
		version     = flag.Bool("version", false, "Show version information")
		debug       = flag.Bool("debug", false, "Enable debug logging")
		healthCheck = flag.Bool("health-check", false, "Perform health check and exit")
	)
	flag.Parse()

	// Show version and exit
	if *version {
		fmt.Printf("%s version %s\n", appName, appVersion)
		os.Exit(0)
	}

	// Generate default config and exit
	if *genConfig {
		configPath := config.GetDefaultConfigPath()
		if *configFile != "" {
			configPath = *configFile
		}
		
		if err := config.CreateDefaultConfig(configPath); err != nil {
			log.Fatalf("Failed to create default config: %v", err)
		}
		
		fmt.Printf("Default configuration created at: %s\n", configPath)
		os.Exit(0)
	}

	// Determine config file path
	configPath := *configFile
	if configPath == "" {
		configPath = config.GetDefaultConfigPath()
		
		// If default config doesn't exist, create it
		if _, err := os.Stat(configPath); os.IsNotExist(err) {
			fmt.Printf("Creating default configuration at: %s\n", configPath)
			if err := config.CreateDefaultConfig(configPath); err != nil {
				log.Fatalf("Failed to create default config: %v", err)
			}
		}
	}

	// Load configuration
	cfg, err := config.LoadConfig(configPath)
	if err != nil {
		log.Fatalf("Failed to load configuration: %v", err)
	}

	// Validate configuration
	if err := cfg.ValidateAndReport(); err != nil {
		log.Fatalf("Configuration validation failed: %v", err)
	}

	// Override debug setting if specified via command line
	if *debug {
		cfg.Settings.Debug = true
	}

	// Setup logging
	if cfg.Settings.Debug {
		log.SetFlags(log.LstdFlags | log.Lshortfile)
		log.Println("Debug logging enabled")
	}

	// Health check mode
	if *healthCheck {
		os.Exit(performHealthCheck(cfg))
	}

	// Print startup information
	log.Printf("Starting %s version %s", appName, appVersion)
	log.Printf("Configuration loaded from: %s", configPath)
	log.Printf("Station: %s.%s", cfg.Settings.Network, cfg.Settings.Station)
	log.Printf("Listening on UDP port: %d", cfg.Settings.Port)

	// Create application context
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	// Create required directories
	if err := createDirectories(cfg); err != nil {
		log.Fatalf("Failed to create directories: %v", err)
	}

	// Initialize and start the application
	app, err := NewApplication(cfg)
	if err != nil {
		log.Fatalf("Failed to initialize application: %v", err)
	}

	// Start application
	if err := app.Start(ctx); err != nil {
		log.Fatalf("Failed to start application: %v", err)
	}
	
	// Start metrics reporting
	go app.startMetricsReporting(ctx)

	// Setup graceful shutdown
	setupGracefulShutdown(app, cancel)

	// Wait for context cancellation
	<-ctx.Done()
	log.Println("Shutting down...")

	// Create shutdown timeout context
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer shutdownCancel()

	// Stop application with timeout
	done := make(chan error, 1)
	go func() {
		done <- app.Stop()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("Error during shutdown: %v", err)
		}
		log.Println("Shutdown complete")
	case <-shutdownCtx.Done():
		log.Println("Shutdown timeout exceeded, forcing exit")
		os.Exit(1)
	}
}

// Application represents the main application
type Application struct {
	config               *config.Config
	broker               *broker.MessageBroker
	producer             *producer.UDPProducer
	stream               *stream.Stream
	alertConsumer        *alert.AlertConsumer
	plotConsumer         *plot.PlotConsumer
	notifyConsumer       *notify.NotificationConsumer
	writerConsumer       *writer.WriterConsumer
	screenshotConsumer   *screenshot.ScreenshotConsumer
	healthChecker        *monitoring.HealthChecker
	startTime            time.Time
}

// NewApplication creates a new application instance
func NewApplication(cfg *config.Config) (*Application, error) {
	app := &Application{
		config:    cfg,
		stream:    stream.NewStream(),
		startTime: time.Now(),
	}

	// Create health checker
	app.healthChecker = monitoring.NewHealthChecker()

	// Register health checks
	app.setupHealthChecks()

	// Create message broker
	app.broker = broker.NewMessageBroker(2048)

	// Create UDP producer
	producerConfig := producer.Config{
		Port:       cfg.Settings.Port,
		BufferSize: 1024,
		Timeout:    10 * time.Second,
	}
	app.producer = producer.NewUDPProducer(producerConfig, app.broker)

	return app, nil
}

// Start starts all application components
func (app *Application) Start(ctx context.Context) error {
	log.Println("Starting message broker...")
	if err := app.broker.Start(); err != nil {
		return fmt.Errorf("failed to start message broker: %v", err)
	}

	log.Println("Starting UDP producer...")
	if err := app.producer.Start(); err != nil {
		return fmt.Errorf("failed to start UDP producer: %v", err)
	}

	// Register a simple consumer for demonstration
	consumer := &SimpleConsumer{id: "simple", stream: app.stream, app: app}
	if err := app.broker.RegisterConsumer(consumer); err != nil {
		return fmt.Errorf("failed to register consumer: %v", err)
	}

	// Register alert consumer if enabled
	if app.config.Alert.Enabled {
		log.Println("Creating alert consumer...")
		var err error
		app.alertConsumer, err = alert.NewAlertConsumer(app.config.Alert, app.broker)
		if err != nil {
			return fmt.Errorf("failed to create alert consumer: %v", err)
		}
		if err := app.broker.RegisterConsumer(app.alertConsumer); err != nil {
			return fmt.Errorf("failed to register alert consumer: %v", err)
		}
		log.Println("Alert consumer registered successfully")
	}

	// Register plot consumer if enabled
	if app.config.Plot.Enabled {
		log.Println("Creating plot consumer...")
		var err error
		app.plotConsumer, err = plot.NewPlotConsumer(app.config.Plot)
		if err != nil {
			return fmt.Errorf("failed to create plot consumer: %v", err)
		}
		
		// Start the plot server
		if err := app.plotConsumer.Start(); err != nil {
			return fmt.Errorf("failed to start plot server: %v", err)
		}
		
		if err := app.broker.RegisterConsumer(app.plotConsumer); err != nil {
			return fmt.Errorf("failed to register plot consumer: %v", err)
		}
		log.Printf("Plot consumer registered successfully, web interface available at http://%s:%d", 
			app.config.Plot.Host, app.config.Plot.Port)
	}

	// Register notification consumer if any notification provider is enabled
	notifyConsumer, err := notify.NewNotificationConsumer(*app.config)
	if err == nil {
		log.Println("Creating notification consumer...")
		app.notifyConsumer = notifyConsumer
		
		// Start the notification manager
		if err := app.notifyConsumer.Start(); err != nil {
			return fmt.Errorf("failed to start notification manager: %v", err)
		}
		
		if err := app.broker.RegisterConsumer(app.notifyConsumer); err != nil {
			return fmt.Errorf("failed to register notification consumer: %v", err)
		}
		log.Println("Notification consumer registered successfully")
	} else {
		log.Printf("Notification consumer disabled: %v", err)
	}

	// Register writer consumer if enabled
	if app.config.Write.Enabled {
		log.Println("Creating writer consumer...")
		writerConsumer, err := writer.NewWriterConsumer(app.config.Write)
		if err != nil {
			return fmt.Errorf("failed to create writer consumer: %v", err)
		}
		app.writerConsumer = writerConsumer

		// Start the writer
		if err := app.writerConsumer.Start(); err != nil {
			return fmt.Errorf("failed to start writer consumer: %v", err)
		}

		if err := app.broker.RegisterConsumer(app.writerConsumer); err != nil {
			return fmt.Errorf("failed to register writer consumer: %v", err)
		}
		log.Printf("Writer consumer registered successfully, writing to: %s", app.config.Write.OutDir)
	}

	// Register screenshot consumer if enabled
	if app.config.Plot.Enabled && app.config.Plot.EqScreenshots {
		log.Println("Creating screenshot consumer...")
		screenshotConsumer, err := screenshot.NewScreenshotConsumer(app.config.Plot)
		if err != nil {
			return fmt.Errorf("failed to create screenshot consumer: %v", err)
		}
		app.screenshotConsumer = screenshotConsumer

		// Start the screenshot capturer
		if err := app.screenshotConsumer.Start(); err != nil {
			return fmt.Errorf("failed to start screenshot consumer: %v", err)
		}

		if err := app.broker.RegisterConsumer(app.screenshotConsumer); err != nil {
			return fmt.Errorf("failed to register screenshot consumer: %v", err)
		}
		log.Println("Screenshot consumer registered successfully")
	}

	// Start health check server
	go func() {
		if err := app.healthChecker.StartHTTPServer(":8081"); err != nil && err != http.ErrServerClosed {
			log.Printf("Health check server error: %v", err)
		}
	}()

	log.Println("Application started successfully")
	log.Println("Health checks available at http://localhost:8081/health")
	log.Println("Waiting for UDP data...")

	// Wait for first data packet
	go func() {
		if err := app.producer.WaitForData(30 * time.Second); err != nil {
			log.Printf("Warning: %v", err)
			log.Printf("Make sure your Raspberry Shake is sending data to this machine")
		} else {
			log.Println("First data packet received!")
		}
	}()

	return nil
}

// Stop stops all application components
func (app *Application) Stop() error {
	log.Println("Stopping UDP producer...")
	if err := app.producer.Stop(); err != nil {
		log.Printf("Error stopping UDP producer: %v", err)
	}

	// Stop consumers with timeouts
	app.stopConsumerWithTimeout("plot consumer", func() error {
		if app.plotConsumer != nil {
			log.Println("Stopping plot consumer...")
			return app.plotConsumer.Stop()
		}
		return nil
	}, 5*time.Second)

	app.stopConsumerWithTimeout("notification consumer", func() error {
		if app.notifyConsumer != nil {
			log.Println("Stopping notification consumer...")
			return app.notifyConsumer.Stop()
		}
		return nil
	}, 5*time.Second)

	app.stopConsumerWithTimeout("writer consumer", func() error {
		if app.writerConsumer != nil {
			log.Println("Stopping writer consumer...")
			return app.writerConsumer.Stop()
		}
		return nil
	}, 5*time.Second)

	app.stopConsumerWithTimeout("screenshot consumer", func() error {
		if app.screenshotConsumer != nil {
			log.Println("Stopping screenshot consumer...")
			return app.screenshotConsumer.Stop()
		}
		return nil
	}, 5*time.Second)

	app.stopConsumerWithTimeout("health checker", func() error {
		if app.healthChecker != nil {
			log.Println("Stopping health checker...")
			return app.healthChecker.Stop()
		}
		return nil
	}, 5*time.Second)

	// Alert consumer will be stopped automatically when broker stops

	log.Println("Stopping message broker...")
	if err := app.broker.Stop(); err != nil {
		log.Printf("Error stopping message broker: %v", err)
	}

	return nil
}

// startMetricsReporting starts periodic metrics reporting
func (app *Application) startMetricsReporting(ctx context.Context) {
	ticker := time.NewTicker(10 * time.Second)
	defer ticker.Stop()
	
	var lastPacketsReceived int64
	
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			if app.producer != nil {
				metrics := app.producer.GetMetrics()
				packetsThisPeriod := metrics.PacketsReceived - lastPacketsReceived
				lastPacketsReceived = metrics.PacketsReceived
				
				log.Printf("UDP Metrics: %d packets received (+%d in last 10s), %d bytes, %d errors", 
					metrics.PacketsReceived, packetsThisPeriod, metrics.BytesReceived, metrics.Errors)
			}
		}
	}
}

// stopConsumerWithTimeout stops a consumer with a timeout
func (app *Application) stopConsumerWithTimeout(name string, stopFunc func() error, timeout time.Duration) {
	done := make(chan error, 1)
	go func() {
		done <- stopFunc()
	}()

	select {
	case err := <-done:
		if err != nil {
			log.Printf("Error stopping %s: %v", name, err)
		}
	case <-time.After(timeout):
		log.Printf("Warning: Timeout stopping %s after %v", name, timeout)
	}
}

// SimpleConsumer is a basic consumer for demonstration
type SimpleConsumer struct {
	id          string
	stream      *stream.Stream
	packetCount int64
	app         *Application
}

func (c *SimpleConsumer) GetID() string {
	return c.id
}

func (c *SimpleConsumer) GetChannelFilters() []string {
	return []string{} // Accept all channels
}

func (c *SimpleConsumer) ProcessEvent(event broker.Event) error {
	switch event.Type {
	case broker.EventData:
		if dataEvent, ok := event.Data.(broker.DataEvent); ok {
			packet := dataEvent.Packet
			
			// Update stream with packet data
			err := c.stream.UpdateFromPacket(packet, "Z0000", "AM")
			if err != nil {
				return fmt.Errorf("failed to update stream: %v", err)
			}
			
			// Log packet reception (only occasionally to avoid spam)
			if packet.GetTimestamp() > 0 && int(packet.GetTimestamp())%10 == 0 {
				log.Printf("Received %s packet: %d samples at %.3f", 
					packet.Channel, len(packet.Data), packet.GetTimestamp())
			}
			
			// Debug: Log every 40th packet for the first minute to verify reception rate
			count := atomic.AddInt64(&c.packetCount, 1)
			if time.Since(c.app.startTime) < time.Minute && count%40 == 0 {
				log.Printf("DEBUG: Packet #%d - %s channel: %d samples", 
					count, packet.Channel, len(packet.Data))
			}
		}
	case broker.EventAlarm:
		log.Printf("ALARM EVENT: %+v", event.Data)
	case broker.EventReset:
		log.Printf("RESET EVENT: %+v", event.Data)
	case broker.EventTerm:
		log.Printf("TERMINATION EVENT: %+v", event.Data)
	}
	
	return nil
}

// setupHealthChecks registers health checks for system components
func (app *Application) setupHealthChecks() {
	// UDP port health check
	app.healthChecker.RegisterCheck("udp_port", monitoring.CreateUDPPortCheck(app.config.Settings.Port))
	
	// Memory health check (512MB limit)
	app.healthChecker.RegisterCheck("memory", monitoring.CreateMemoryCheck(512))
	
	// Data directory health check
	if app.config.Write.Enabled {
		app.healthChecker.RegisterCheck("data_directory", monitoring.CreateDiskSpaceCheck(app.config.Write.OutDir, 1))
	}
	
	// Screenshot directory health check
	if app.config.Plot.Enabled && app.config.Plot.EqScreenshots {
		app.healthChecker.RegisterCheck("screenshot_directory", monitoring.CreateDiskSpaceCheck("./screenshots", 1))
	}
}

// createDirectories creates necessary directories
func createDirectories(cfg *config.Config) error {
	dirs := []string{
		cfg.Settings.OutDir,
		cfg.Settings.DataDir,
		cfg.Settings.ScreenshotDir,
		cfg.Settings.LogDir,
	}

	for _, dir := range dirs {
		if dir == "" {
			continue
		}
		
		if err := os.MkdirAll(dir, 0755); err != nil {
			return fmt.Errorf("failed to create directory %s: %v", dir, err)
		}
	}

	return nil
}

// setupGracefulShutdown sets up graceful shutdown handling
func setupGracefulShutdown(app *Application, cancel context.CancelFunc) {
	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	go func() {
		sig := <-c
		log.Printf("Received signal %v, initiating graceful shutdown...", sig)
		cancel()
	}()
}

// performHealthCheck performs a health check and returns exit code
func performHealthCheck(cfg *config.Config) int {
	log.Println("Performing health check...")

	// Validate configuration
	if err := cfg.ValidateAndReport(); err != nil {
		log.Printf("Health check failed: %v", err)
		return 1
	}

	// Check if we can create required directories
	if err := createDirectories(cfg); err != nil {
		log.Printf("Health check failed: %v", err)
		return 1
	}

	// Create health checker for validation
	healthChecker := monitoring.NewHealthChecker()
	
	// Register basic health checks
	healthChecker.RegisterCheck("udp_port", monitoring.CreateUDPPortCheck(cfg.Settings.Port))
	healthChecker.RegisterCheck("memory", monitoring.CreateMemoryCheck(512))
	if cfg.Write.Enabled {
		healthChecker.RegisterCheck("data_directory", monitoring.CreateDiskSpaceCheck(cfg.Write.OutDir, 1))
	}

	// Run health checks
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	healthChecker.RunChecks(ctx)
	health := healthChecker.GetHealth()

	if health.Status == monitoring.HealthStatusUnhealthy {
		log.Printf("Health check failed: %s", health.Status)
		return 1
	}

	log.Printf("Health check passed: %s", health.Status)
	return 0
}