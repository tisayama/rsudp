// Package main provides the main entry point for gorsudp
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/tisayama/gorsudp/internal/broker"
	"github.com/tisayama/gorsudp/internal/producer"
	"github.com/tisayama/gorsudp/pkg/config"
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

	// Setup graceful shutdown
	setupGracefulShutdown(app, cancel)

	// Wait for context cancellation
	<-ctx.Done()
	log.Println("Shutting down...")

	// Stop application
	if err := app.Stop(); err != nil {
		log.Printf("Error during shutdown: %v", err)
	}

	log.Println("Shutdown complete")
}

// Application represents the main application
type Application struct {
	config   *config.Config
	broker   *broker.MessageBroker
	producer *producer.UDPProducer
	stream   *stream.Stream
}

// NewApplication creates a new application instance
func NewApplication(cfg *config.Config) (*Application, error) {
	app := &Application{
		config: cfg,
		stream: stream.NewStream(),
	}

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
	consumer := &SimpleConsumer{id: "simple", stream: app.stream}
	if err := app.broker.RegisterConsumer(consumer); err != nil {
		return fmt.Errorf("failed to register consumer: %v", err)
	}

	log.Println("Application started successfully")
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

	log.Println("Stopping message broker...")
	if err := app.broker.Stop(); err != nil {
		log.Printf("Error stopping message broker: %v", err)
	}

	return nil
}

// SimpleConsumer is a basic consumer for demonstration
type SimpleConsumer struct {
	id     string
	stream *stream.Stream
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

	// Check if we can create required directories
	if err := createDirectories(cfg); err != nil {
		log.Printf("Health check failed: %v", err)
		return 1
	}

	// Check if we can bind to the UDP port
	app, err := NewApplication(cfg)
	if err != nil {
		log.Printf("Health check failed: %v", err)
		return 1
	}

	if err := app.producer.Start(); err != nil {
		log.Printf("Health check failed: %v", err)
		return 1
	}

	app.producer.Stop()

	log.Println("Health check passed")
	return 0
}