package plot

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"

	"github.com/gorilla/mux"
)

// PlotServer manages the web server for real-time plotting
type PlotServer struct {
	config       *PlotConfig
	wsManager    *WebSocketManager
	server       *http.Server
	router       *mux.Router
	dataBuffer   *CircularBuffer
	systemStats  *SystemStats
}

// CircularBuffer holds recent plot data for new clients
type CircularBuffer struct {
	data     []PlotData
	capacity int
	index    int
	full     bool
}

// SystemStats tracks system statistics
type SystemStats struct {
	StartTime       time.Time
	PacketsReceived int64
	AlertsTriggered int64
	ActiveChannels  map[string]time.Time
}

// NewPlotServer creates a new plot server
func NewPlotServer(config *PlotConfig) *PlotServer {
	wsManager := NewWebSocketManager()
	
	server := &PlotServer{
		config:      config,
		wsManager:   wsManager,
		dataBuffer:  NewCircularBuffer(1000), // Keep last 1000 data points
		systemStats: &SystemStats{
			StartTime:      time.Now(),
			ActiveChannels: make(map[string]time.Time),
		},
	}
	
	server.setupRoutes()
	
	return server
}

// NewCircularBuffer creates a new circular buffer
func NewCircularBuffer(capacity int) *CircularBuffer {
	return &CircularBuffer{
		data:     make([]PlotData, capacity),
		capacity: capacity,
		index:    0,
		full:     false,
	}
}

// Add adds data to the circular buffer
func (cb *CircularBuffer) Add(data PlotData) {
	cb.data[cb.index] = data
	cb.index = (cb.index + 1) % cb.capacity
	if cb.index == 0 {
		cb.full = true
	}
}

// GetRecent returns recent data from the buffer
func (cb *CircularBuffer) GetRecent(count int) []PlotData {
	if count > cb.capacity {
		count = cb.capacity
	}
	
	size := cb.index
	if cb.full {
		size = cb.capacity
	}
	
	if count > size {
		count = size
	}
	
	result := make([]PlotData, count)
	start := cb.index - count
	if start < 0 {
		if cb.full {
			// Wrap around
			firstPart := cb.capacity + start
			copy(result[0:count+start], cb.data[firstPart:])
			copy(result[count+start:], cb.data[0:cb.index])
		} else {
			// Not enough data
			copy(result, cb.data[0:cb.index])
		}
	} else {
		copy(result, cb.data[start:cb.index])
	}
	
	return result
}

// setupRoutes sets up HTTP routes
func (server *PlotServer) setupRoutes() {
	server.router = mux.NewRouter()
	
	// WebSocket endpoint
	server.router.HandleFunc("/ws", server.wsManager.HandleWebSocket)
	
	// API endpoints
	server.router.HandleFunc("/api/status", server.handleStatus).Methods("GET")
	server.router.HandleFunc("/api/config", server.handleConfig).Methods("GET")
	server.router.HandleFunc("/api/channels", server.handleChannels).Methods("GET")
	server.router.HandleFunc("/api/recent/{channel}", server.handleRecentData).Methods("GET")
	
	// Static files (will be implemented later with embedded files)
	server.router.PathPrefix("/static/").Handler(http.StripPrefix("/static/", http.FileServer(http.Dir("./web/static/"))))
	server.router.HandleFunc("/", server.handleIndex)
	
	// Setup CORS middleware
	server.router.Use(corsMiddleware)
}

// Start starts the plot server
func (server *PlotServer) Start() error {
	server.wsManager.Start()
	
	addr := fmt.Sprintf("%s:%d", server.config.Host, server.config.Port)
	server.server = &http.Server{
		Addr:    addr,
		Handler: server.router,
	}
	
	log.Printf("Starting plot server on %s", addr)
	
	go func() {
		if err := server.server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Printf("Plot server error: %v", err)
		}
	}()
	
	// Start periodic status broadcasting
	go server.broadcastStatusPeriodically()
	
	return nil
}

// Stop stops the plot server
func (server *PlotServer) Stop() error {
	log.Println("Stopping plot server...")
	
	server.wsManager.Stop()
	
	if server.server != nil {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		return server.server.Shutdown(ctx)
	}
	
	return nil
}

// broadcastStatusPeriodically sends status updates via WebSocket every 5 seconds
func (server *PlotServer) broadcastStatusPeriodically() {
	ticker := time.NewTicker(5 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case <-ticker.C:
			// Create status message
			activeChannels := make([]string, 0, len(server.systemStats.ActiveChannels))
			for channel := range server.systemStats.ActiveChannels {
				activeChannels = append(activeChannels, channel)
			}
			
			status := SystemStatus{
				Timestamp:       time.Now(),
				ActiveChannels:  activeChannels,
				ClientCount:     server.wsManager.GetClientCount(),
				PacketsReceived: server.systemStats.PacketsReceived,
				AlertsTriggered: server.systemStats.AlertsTriggered,
				Uptime:          time.Since(server.systemStats.StartTime).String(),
			}
			
			// Broadcast status
			message := WebSocketMessage{
				Type: MessageTypeSystemStatus,
				Data: status,
			}
			
			server.wsManager.Broadcast(message)
		}
	}
}

// AddPlotData adds new plot data and broadcasts it
func (server *PlotServer) AddPlotData(data PlotData) {
	server.dataBuffer.Add(data)
	server.systemStats.PacketsReceived++
	server.systemStats.ActiveChannels[data.Channel] = data.Timestamp
	
	message := WebSocketMessage{
		Type: MessageTypePlotData,
		Data: data,
	}
	
	server.wsManager.Broadcast(message)
}

// AddAlert broadcasts an alert message
func (server *PlotServer) AddAlert(alert AlertMessage) {
	server.systemStats.AlertsTriggered++
	
	message := WebSocketMessage{
		Type: MessageTypeAlert,
		Data: alert,
	}
	
	server.wsManager.Broadcast(message)
}

// HTTP Handlers

func (server *PlotServer) handleIndex(w http.ResponseWriter, r *http.Request) {
	html := `<!DOCTYPE html>
<html>
<head>
    <title>GoRSUDP - Real-time Seismic Monitoring</title>
    <meta charset="utf-8">
    <meta name="viewport" content="width=device-width, initial-scale=1">
    <script src="https://cdn.jsdelivr.net/npm/chart.js"></script>
    <script src="https://cdn.jsdelivr.net/npm/chartjs-adapter-date-fns/dist/chartjs-adapter-date-fns.bundle.min.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f0f0f0; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { text-align: center; margin-bottom: 30px; }
        .status { display: flex; justify-content: space-between; margin-bottom: 20px; }
        .status-item { text-align: center; padding: 10px; background: #f8f9fa; border-radius: 4px; }
        .plot-container { height: 350px; border: 1px solid #ddd; margin-bottom: 20px; background: white; position: relative; padding: 10px; }
        .plot-container h3 { margin: 0 0 10px 0; text-align: center; font-size: 16px; }
        .channel-tabs { display: flex; margin-bottom: 10px; }
        .channel-tab { padding: 8px 16px; background: #f8f9fa; border: 1px solid #ddd; cursor: pointer; margin-right: 5px; }
        .channel-tab.active { background: #007bff; color: white; }
        .plot-controls { margin-bottom: 10px; }
        .plot-controls label { margin-right: 20px; cursor: pointer; }
        .alert { padding: 10px; margin: 10px 0; border-radius: 4px; }
        .alert-info { background: #d1ecf1; border: 1px solid #bee5eb; color: #0c5460; }
        .alert-warning { background: #fff3cd; border: 1px solid #ffeaa7; color: #856404; }
        .alert-danger { background: #f8d7da; border: 1px solid #f5c6cb; color: #721c24; }
    </style>
</head>
<body>
    <div class="container">
        <div class="header">
            <h1>GoRSUDP Real-time Seismic Monitoring</h1>
            <div id="connection-status" class="alert alert-info">Connecting to server...</div>
        </div>
        
        <div class="status">
            <div class="status-item">
                <strong>Status</strong><br>
                <span id="system-status">Initializing...</span>
            </div>
            <div class="status-item">
                <strong>Active Channels</strong><br>
                <span id="active-channels">-</span>
            </div>
            <div class="status-item">
                <strong>Packets Received</strong><br>
                <span id="packets-received">0</span>
            </div>
            <div class="status-item">
                <strong>Alerts Triggered</strong><br>
                <span id="alerts-triggered">0</span>
            </div>
        </div>
        
        <div class="channel-tabs" id="channel-tabs"></div>
        
        <div class="plot-container">
            <h3>Waveform</h3>
            <canvas id="seismic-chart"></canvas>
        </div>
        
        <div class="plot-container">
            <h3>Spectrogram</h3>
            <div style="position: relative; width: 100%; height: calc(100% - 30px);">
                <canvas id="spectrogram-canvas" style="border: 1px solid #ddd; width: 100%; height: 100%;"></canvas>
                <div id="spectrogram-axis" style="position: absolute; left: 0; top: 0; pointer-events: none;"></div>
            </div>
        </div>
        
        <div id="alerts-container"></div>
    </div>

    <script>
        // Chart.js setup
        let waveformChart = null;
        let spectrogramCanvas = null;
        let spectrogramCtx = null;
        let spectrogramData = [];
        let channelData = {};
        let activeChannel = null;
        const maxDataPoints = 2000; // Increased for better spectrogram
        const sampleRate = 100; // Hz
        const spectrogramWindowSize = 256; // FFT window size
        const spectrogramOverlap = 128; // Overlap between windows
        
        // Configuration from server (will be set dynamically)
        let plotConfig = {};
        let maxFreq = 50; // Default, will be updated from config
        let minFreq = 0;  // Default, will be updated from config
        let spectrogramHeight = 300; // Will be updated dynamically
        let spectrogramWidth = 1200;  // Will be updated dynamically

        // Initialize charts
        function initCharts() {
            initWaveformChart();
            initSpectrogramCanvas();
        }

        function initWaveformChart() {
            const ctx = document.getElementById('seismic-chart').getContext('2d');
            waveformChart = new Chart(ctx, {
                type: 'line',
                data: {
                    datasets: [{
                        label: 'Seismic Data',
                        data: [],
                        borderColor: 'rgb(75, 192, 192)',
                        backgroundColor: 'rgba(75, 192, 192, 0.1)',
                        borderWidth: 1,
                        fill: false,
                        pointRadius: 0
                    }]
                },
                options: {
                    responsive: true,
                    maintainAspectRatio: false,
                    scales: {
                        x: {
                            type: 'time',
                            time: {
                                displayFormats: {
                                    second: 'mm:ss',
                                    minute: 'HH:mm'
                                },
                                unit: 'second',
                                stepSize: 10 // Show ticks every 10 seconds
                            },
                            title: {
                                display: true,
                                text: 'Time'
                            }
                        },
                        y: {
                            title: {
                                display: true,
                                text: 'Velocity (μm/s)'
                            },
                            ticks: {
                                callback: function(value) {
                                    // Data is already in μm/s, just format appropriately
                                    if (Math.abs(value) < 0.001 && value !== 0) {
                                        return value.toExponential(2);
                                    }
                                    return value.toFixed(2);
                                }
                            }
                        }
                    },
                    plugins: {
                        legend: {
                            display: true
                        }
                    },
                    animation: false
                }
            });
        }

        function initSpectrogramCanvas() {
            spectrogramCanvas = document.getElementById('spectrogram-canvas');
            spectrogramCtx = spectrogramCanvas.getContext('2d');
            
            // Set high resolution canvas size
            const container = spectrogramCanvas.parentElement;
            const containerWidth = container.clientWidth - 2; // Account for border
            const containerHeight = container.clientHeight - 2; // Account for border
            
            // High resolution for crisp display
            const pixelRatio = window.devicePixelRatio || 1;
            spectrogramWidth = containerWidth * pixelRatio;
            spectrogramHeight = containerHeight * pixelRatio;
            
            spectrogramCanvas.width = spectrogramWidth;
            spectrogramCanvas.height = spectrogramHeight;
            
            // Scale the canvas back down using CSS
            spectrogramCanvas.style.width = containerWidth + 'px';
            spectrogramCanvas.style.height = containerHeight + 'px';
            
            // Scale the drawing context to match the device pixel ratio
            spectrogramCtx.scale(pixelRatio, pixelRatio);
            
            // Initialize with black background
            spectrogramCtx.fillStyle = 'black';
            spectrogramCtx.fillRect(0, 0, containerWidth, containerHeight);
            
            // Update dimensions for internal use (CSS pixels)
            spectrogramWidth = containerWidth;
            spectrogramHeight = containerHeight;
            
            // Draw frequency axis labels
            drawSpectrogramAxes();
        }
        
        function drawSpectrogramAxes() {
            const axisDiv = document.getElementById('spectrogram-axis');
            axisDiv.innerHTML = '';
            
            // Determine appropriate frequency step
            const freqRange = maxFreq - minFreq;
            let step = 1;
            if (freqRange > 50) step = 10;
            else if (freqRange > 20) step = 5;
            else if (freqRange > 10) step = 2;
            else step = 1;
            
            // Create frequency labels
            for (let freq = minFreq; freq <= maxFreq; freq += step) {
                const normalizedY = (freq - minFreq) / (maxFreq - minFreq);
                const y = spectrogramHeight - normalizedY * spectrogramHeight;
                const label = document.createElement('div');
                label.style.position = 'absolute';
                label.style.left = '-30px';
                label.style.top = y + 'px';
                label.style.fontSize = '10px';
                label.style.color = '#666';
                label.textContent = freq.toFixed(1) + ' Hz';
                axisDiv.appendChild(label);
            }
        }
        
        function viridisColormap(value) {
            // Viridis colormap approximation (value should be 0-1)
            value = Math.max(0, Math.min(1, value));
            
            const r = Math.floor(255 * (0.267004 + value * (0.127568 - 0.267004 + value * (0.893570 - 0.127568))));
            const g = Math.floor(255 * (0.004874 + value * (0.566949 - 0.004874 + value * (0.334200 - 0.566949))));
            const b = Math.floor(255 * (0.329415 + value * (0.550556 - 0.329415 + value * (0.108365 - 0.550556))));
            
            return 'rgb(' + r + ',' + g + ',' + b + ')';
        }

        // Improved FFT implementation
        function fft(signal) {
            const N = signal.length;
            if (N <= 1) return signal;
            
            // Ensure power of 2
            const nextPow2 = Math.pow(2, Math.ceil(Math.log2(N)));
            const paddedSignal = [...signal];
            while (paddedSignal.length < nextPow2) {
                paddedSignal.push(0);
            }
            
            return fftRecursive(paddedSignal);
        }
        
        function fftRecursive(signal) {
            const N = signal.length;
            if (N <= 1) return signal.map(x => ({real: x, imag: 0}));
            
            // Divide
            const even = [];
            const odd = [];
            for (let i = 0; i < N; i++) {
                if (i % 2 === 0) {
                    even.push(signal[i]);
                } else {
                    odd.push(signal[i]);
                }
            }
            
            // Conquer
            const evenFFT = fftRecursive(even);
            const oddFFT = fftRecursive(odd);
            
            // Combine
            const result = new Array(N);
            for (let k = 0; k < N / 2; k++) {
                const angle = -2 * Math.PI * k / N;
                const cos = Math.cos(angle);
                const sin = Math.sin(angle);
                
                const tReal = cos * oddFFT[k].real - sin * oddFFT[k].imag;
                const tImag = sin * oddFFT[k].real + cos * oddFFT[k].imag;
                
                result[k] = {
                    real: evenFFT[k].real + tReal,
                    imag: evenFFT[k].imag + tImag
                };
                result[k + N / 2] = {
                    real: evenFFT[k].real - tReal,
                    imag: evenFFT[k].imag - tImag
                };
            }
            
            return result;
        }
        
        function updateSpectrogramChart() {
            if (!spectrogramCtx || !activeChannel || !channelData[activeChannel]) return;
            
            const channelInfo = channelData[activeChannel];
            const data = channelInfo.data;
            const times = channelInfo.times;
            
            if (data.length < spectrogramWindowSize) return;
            
            // Shift existing spectrogram data to the left (4 pixels at a time)
            const imageData = spectrogramCtx.getImageData(4, 0, spectrogramWidth - 4, spectrogramHeight);
            spectrogramCtx.clearRect(0, 0, spectrogramWidth, spectrogramHeight);
            spectrogramCtx.putImageData(imageData, 0, 0);
            
            // Calculate new column of spectrogram data
            const latestData = data.slice(-spectrogramWindowSize);
            
            // Apply Hanning window
            const windowedData = latestData.map((val, idx) => {
                const hanningFactor = 0.5 * (1 - Math.cos(2 * Math.PI * idx / (spectrogramWindowSize - 1)));
                return val * hanningFactor;
            });
            
            // Compute FFT
            const fftResult = fft(windowedData);
            
            // Draw new column
            const freqResolution = sampleRate / spectrogramWindowSize;
            const totalFreqBins = Math.floor(spectrogramWindowSize / 2);
            
            // Calculate power spectrum and find min/max for normalization
            const powerSpectrum = [];
            let minPower = Infinity;
            let maxPower = -Infinity;
            
            for (let k = 0; k < totalFreqBins; k++) {
                const frequency = k * freqResolution;
                
                // Only include frequencies within our specified range
                if (frequency >= minFreq && frequency <= maxFreq) {
                    const magnitude = fftResult[k].real * fftResult[k].real + fftResult[k].imag * fftResult[k].imag;
                    const power = Math.log10(Math.max(magnitude / spectrogramWindowSize, 1e-10));
                    powerSpectrum.push({frequency, power});
                    minPower = Math.min(minPower, power);
                    maxPower = Math.max(maxPower, power);
                }
            }
            
            // Draw frequency bins for the new time column with high y-axis resolution
            // Use smaller frequency bins for higher resolution - aim for 2 pixel height bins
            const targetFreqBinHeight = 2;
            const numFreqBins = Math.floor(spectrogramHeight / targetFreqBinHeight);
            const actualFreqBinHeight = spectrogramHeight / numFreqBins;
            
            // Create interpolated frequency spectrum for higher resolution
            for (let i = 0; i < numFreqBins; i++) {
                const freqRatio = i / (numFreqBins - 1);
                const targetFreq = minFreq + freqRatio * (maxFreq - minFreq);
                
                // Find interpolated power value for this frequency
                let interpolatedPower = 0;
                if (powerSpectrum.length > 1) {
                    // Find the two closest frequency bins for interpolation
                    let lowerIndex = -1;
                    let upperIndex = -1;
                    
                    for (let j = 0; j < powerSpectrum.length - 1; j++) {
                        if (powerSpectrum[j].frequency <= targetFreq && powerSpectrum[j + 1].frequency >= targetFreq) {
                            lowerIndex = j;
                            upperIndex = j + 1;
                            break;
                        }
                    }
                    
                    if (lowerIndex >= 0 && upperIndex >= 0) {
                        // Linear interpolation
                        const lowerFreq = powerSpectrum[lowerIndex].frequency;
                        const upperFreq = powerSpectrum[upperIndex].frequency;
                        const lowerPower = powerSpectrum[lowerIndex].power;
                        const upperPower = powerSpectrum[upperIndex].power;
                        
                        const ratio = (targetFreq - lowerFreq) / (upperFreq - lowerFreq);
                        interpolatedPower = lowerPower + ratio * (upperPower - lowerPower);
                    } else if (powerSpectrum.length > 0) {
                        // Use nearest neighbor if interpolation isn't possible
                        const nearest = powerSpectrum.reduce((prev, curr) => 
                            Math.abs(curr.frequency - targetFreq) < Math.abs(prev.frequency - targetFreq) ? curr : prev
                        );
                        interpolatedPower = nearest.power;
                    }
                }
                
                const y = spectrogramHeight - (i + 1) * actualFreqBinHeight;
                
                // Normalize power to 0-1 range
                const normalizedPower = maxPower > minPower ? (interpolatedPower - minPower) / (maxPower - minPower) : 0;
                const color = viridisColormap(normalizedPower);
                
                spectrogramCtx.fillStyle = color;
                spectrogramCtx.fillRect(spectrogramWidth - 4, y, 4, actualFreqBinHeight);
            }
        }

        // WebSocket connection
        const protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:';
        const wsUrl = protocol + '//' + window.location.host + '/ws';
        let ws = null;
        let reconnectInterval = null;

        function connect() {
            try {
                ws = new WebSocket(wsUrl);
                
                ws.onopen = function(event) {
                    console.log('Connected to WebSocket');
                    document.getElementById('connection-status').textContent = 'Connected to server';
                    document.getElementById('connection-status').className = 'alert alert-info';
                    
                    if (reconnectInterval) {
                        clearInterval(reconnectInterval);
                        reconnectInterval = null;
                    }
                };
                
                ws.onmessage = function(event) {
                    try {
                        const message = JSON.parse(event.data);
                        handleMessage(message);
                    } catch (e) {
                        console.error('Error parsing message:', e);
                    }
                };
                
                ws.onclose = function(event) {
                    console.log('WebSocket connection closed');
                    document.getElementById('connection-status').textContent = 'Disconnected from server';
                    document.getElementById('connection-status').className = 'alert alert-warning';
                    
                    // Attempt to reconnect
                    if (!reconnectInterval) {
                        reconnectInterval = setInterval(connect, 5000);
                    }
                };
                
                ws.onerror = function(error) {
                    console.error('WebSocket error:', error);
                    document.getElementById('connection-status').textContent = 'Connection error';
                    document.getElementById('connection-status').className = 'alert alert-danger';
                };
                
            } catch (e) {
                console.error('Failed to create WebSocket connection:', e);
                if (!reconnectInterval) {
                    reconnectInterval = setInterval(connect, 5000);
                }
            }
        }

        function handleMessage(message) {
            switch (message.type) {
                case 'plot_data':
                    addPlotData(message.data);
                    updateStatus();
                    break;
                case 'alert':
                    addAlert(message.data);
                    break;
                case 'system_status':
                    updateSystemStatus(message.data);
                    break;
                case 'config':
                    updateConfig(message.data);
                    break;
            }
        }

        function updateConfig(config) {
            plotConfig = config;
            console.log('Plot config updated:', config);
            
            // Update frequency range for spectrogram
            if (config.spectrogram_freq_range) {
                minFreq = config.lower_limit || 0;
                maxFreq = config.upper_limit || 50;
            } else {
                minFreq = 0;
                maxFreq = 50;
            }
            
            // Update spectrogram axes
            drawSpectrogramAxes();
        }

        function addPlotData(data) {
            const channel = data.channel;
            
            // Initialize channel data if not exists
            if (!channelData[channel]) {
                channelData[channel] = {
                    times: [],
                    data: [],
                    units: data.units || 'unknown'
                };
                addChannelTab(channel);
            }
            
            // Add new data points with proper timestamps
            const timestamp = new Date(data.timestamp);
            const samples = data.samples;
            const sampleRate = data.sample_rate || 100;
            
            for (let i = 0; i < samples.length; i++) {
                const sampleTime = new Date(timestamp.getTime() + (i * 1000 / sampleRate));
                channelData[channel].times.push(sampleTime);
                channelData[channel].data.push(samples[i]);
            }
            
            // Keep only last maxDataPoints
            if (channelData[channel].data.length > maxDataPoints) {
                const excess = channelData[channel].data.length - maxDataPoints;
                channelData[channel].times.splice(0, excess);
                channelData[channel].data.splice(0, excess);
            }
            
            // Update chart if this is the active channel
            if (activeChannel === channel) {
                updateChart();
            }
        }

        function addChannelTab(channel) {
            const tabsContainer = document.getElementById('channel-tabs');
            const tab = document.createElement('div');
            tab.className = 'channel-tab';
            tab.textContent = channel;
            tab.onclick = () => switchChannel(channel);
            
            tabsContainer.appendChild(tab);
            
            // Set as active if first channel
            if (!activeChannel) {
                switchChannel(channel);
            }
        }

        function switchChannel(channel) {
            // Update tab appearance
            document.querySelectorAll('.channel-tab').forEach(tab => {
                tab.classList.toggle('active', tab.textContent === channel);
            });
            
            activeChannel = channel;
            updateChart();
        }

        function updateChart() {
            if (!waveformChart || !activeChannel || !channelData[activeChannel]) return;
            
            const channelInfo = channelData[activeChannel];
            
            // Calculate mean of current data (like Python implementation)
            const mean = channelInfo.data.length > 0 ? 
                channelInfo.data.reduce((sum, val) => sum + val, 0) / channelInfo.data.length : 0;
            
            // Create data points with mean-centered values converted to μm/s
            const chartData = [];
            for (let i = 0; i < channelInfo.times.length; i++) {
                chartData.push({
                    x: channelInfo.times[i],
                    y: (channelInfo.data[i] - mean) * 1000000 // Convert m/s to μm/s
                });
            }
            
            waveformChart.data.datasets[0].data = chartData;
            waveformChart.data.datasets[0].label = activeChannel + ' Seismic Data (mean-centered)';
            
            // Update Y-axis label with appropriate units
            const units = formatUnits(channelInfo.data, channelInfo.units);
            waveformChart.options.scales.y.title.text = 'Velocity (' + units + ')';
            
            // Set Y-axis range based on centered data
            if (chartData.length > 0) {
                const centeredValues = chartData.map(point => point.y);
                const maxAbs = Math.max(...centeredValues.map(Math.abs));
                const padding = maxAbs * 0.1; // 10% padding
                
                waveformChart.options.scales.y.min = -maxAbs - padding;
                waveformChart.options.scales.y.max = maxAbs + padding;
            }
            
            waveformChart.update('none');
            
            // Always update spectrogram when waveform updates
            updateSpectrogramChart();
        }

        function formatUnits(data, originalUnits) {
            if (!data || data.length === 0) return originalUnits;
            
            // Calculate typical amplitude
            const maxAbs = Math.max(...data.map(Math.abs));
            
            // Auto-scale units based on magnitude
            if (maxAbs > 1) {
                return 'm/s';
            } else if (maxAbs > 0.001) {
                return 'mm/s';
            } else if (maxAbs > 0.000001) {
                return 'μm/s';
            } else {
                return 'nm/s';
            }
        }

        function updateStatus() {
            // Status updates are now handled via WebSocket
            // This function is kept for compatibility but no longer polls
        }

        function updateSystemStatus(status) {
            document.getElementById('system-status').textContent = 'Running';
            document.getElementById('active-channels').textContent = status.active_channels.join(', ') || 'None';
            document.getElementById('packets-received').textContent = status.packets_received;
            document.getElementById('alerts-triggered').textContent = status.alerts_triggered;
        }

        function addAlert(alert) {
            const container = document.getElementById('alerts-container');
            const alertDiv = document.createElement('div');
            alertDiv.className = 'alert alert-danger';
            alertDiv.innerHTML = '<strong>EARTHQUAKE ALERT!</strong> ' + 
                'Channel: ' + alert.channel + ', ' +
                'STA/LTA: ' + alert.stalta_ratio.toFixed(2) + ', ' +
                'Time: ' + new Date(alert.timestamp).toLocaleString();
            
            container.insertBefore(alertDiv, container.firstChild);
            
            // Remove old alerts (keep last 10)
            while (container.children.length > 10) {
                container.removeChild(container.lastChild);
            }
        }

        // Initialize
        document.addEventListener('DOMContentLoaded', function() {
            initCharts();
            connect();
            
            // Request config from server
            setTimeout(() => {
                fetch('/api/config')
                    .then(response => response.json())
                    .then(config => updateConfig(config))
                    .catch(error => console.error('Error fetching config:', error));
            }, 1000);
            
            // Status updates are now handled via WebSocket, no need for polling
        });
    </script>
</body>
</html>`
	
	w.Header().Set("Content-Type", "text/html")
	w.Write([]byte(html))
}

func (server *PlotServer) handleStatus(w http.ResponseWriter, r *http.Request) {
	activeChannels := make([]string, 0, len(server.systemStats.ActiveChannels))
	for channel := range server.systemStats.ActiveChannels {
		activeChannels = append(activeChannels, channel)
	}
	
	status := SystemStatus{
		Timestamp:       time.Now(),
		ActiveChannels:  activeChannels,
		ClientCount:     server.wsManager.GetClientCount(),
		PacketsReceived: server.systemStats.PacketsReceived,
		AlertsTriggered: server.systemStats.AlertsTriggered,
		Uptime:          time.Since(server.systemStats.StartTime).String(),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(status)
}

func (server *PlotServer) handleConfig(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(server.config)
}

func (server *PlotServer) handleChannels(w http.ResponseWriter, r *http.Request) {
	channels := make([]string, 0, len(server.systemStats.ActiveChannels))
	for channel := range server.systemStats.ActiveChannels {
		channels = append(channels, channel)
	}
	
	response := map[string]interface{}{
		"channels": channels,
		"count":    len(channels),
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(response)
}

func (server *PlotServer) handleRecentData(w http.ResponseWriter, r *http.Request) {
	vars := mux.Vars(r)
	channel := vars["channel"]
	
	// Get recent data for the specific channel
	allData := server.dataBuffer.GetRecent(100)
	channelData := make([]PlotData, 0)
	
	for _, data := range allData {
		if data.Channel == channel || channel == "all" {
			channelData = append(channelData, data)
		}
	}
	
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(channelData)
}

// corsMiddleware adds CORS headers
func corsMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Access-Control-Allow-Origin", "*")
		w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
		w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization")
		
		if r.Method == "OPTIONS" {
			w.WriteHeader(http.StatusOK)
			return
		}
		
		next.ServeHTTP(w, r)
	})
}