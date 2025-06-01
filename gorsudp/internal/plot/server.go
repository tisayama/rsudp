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
	config      *PlotConfig
	wsManager   *WebSocketManager
	server      *http.Server
	router      *mux.Router
	dataBuffer  *CircularBuffer
	systemStats *SystemStats
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
		config:     config,
		wsManager:  wsManager,
		dataBuffer: NewCircularBuffer(1000), // Keep last 1000 data points
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
    <script src="https://d3js.org/d3.v7.min.js"></script>
    <style>
        body { font-family: Arial, sans-serif; margin: 0; padding: 20px; background: #f0f0f0; }
        .container { max-width: 1200px; margin: 0 auto; background: white; padding: 20px; border-radius: 8px; box-shadow: 0 2px 10px rgba(0,0,0,0.1); }
        .header { text-align: center; margin-bottom: 30px; }
        .status { display: flex; justify-content: space-between; margin-bottom: 20px; }
        .status-item { text-align: center; padding: 10px; background: #f8f9fa; border-radius: 4px; }
        .plot-container { height: 350px; border: 1px solid #ddd; margin-bottom: 20px; background: white; position: relative; padding: 10px; }
        .plot-container h3 { margin: 0 0 10px 0; text-align: center; font-size: 16px; }
        .plot-svg { width: 100%; height: calc(100% - 30px); }
        .axis { font-size: 12px; }
        .axis path, .axis line { fill: none; stroke: #000; shape-rendering: crispEdges; }
        .waveform-path { fill: none; stroke: #1f77b4; stroke-width: 1.5px; }
        .grid-line { stroke: #ddd; stroke-dasharray: 2,2; }
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
            <svg id="waveform-svg" class="plot-svg"></svg>
        </div>
        
        <div class="plot-container">
            <h3>Spectrogram</h3>
            <svg id="spectrogram-svg" class="plot-svg"></svg>
        </div>
        
        <div id="alerts-container"></div>
    </div>

    <script>
        // D3.js setup
        let waveformSvg = null;
        let spectrogramSvg = null;
        let waveformData = [];
        let spectrogramData = [];
        let channelData = {};
        let activeChannel = null;
        
        // Plot dimensions and margins
        const margin = {top: 20, right: 20, bottom: 40, left: 60};
        let width, height;
        
        // Scales
        let xScale, yScaleWaveform, yScaleSpectrogram;
        let colorScale;
        
        // Configuration
        const sampleRate = 100; // Hz
        const spectrogramWindowSize = 256;
        const spectrogramOverlap = 128;
        const maxSpectrogramPoints = 2000;
        const timeWindowSeconds = 120; // Show last 120 seconds
        
        // Server configuration
        let plotConfig = {};
        let maxFreq = 50;
        let minFreq = 0;

        // Initialize charts
        function initCharts() {
            initDimensions();
            initWaveformChart();
            initSpectrogramChart();
        }
        
        function initDimensions() {
            const container = document.querySelector('.plot-container');
            const containerRect = container.getBoundingClientRect();
            width = containerRect.width - margin.left - margin.right - 40; // Account for padding
            height = 280 - margin.top - margin.bottom; // Fixed height minus margins
        }

        function initWaveformChart() {
            // Clear any existing SVG
            d3.select("#waveform-svg").selectAll("*").remove();
            
            // Create SVG
            waveformSvg = d3.select("#waveform-svg")
                .attr("width", width + margin.left + margin.right)
                .attr("height", height + margin.top + margin.bottom);
            
            const g = waveformSvg.append("g")
                .attr("transform", "translate(" + margin.left + "," + margin.top + ")");
            
            // Initialize scales
            const now = new Date();
            const past = new Date(now.getTime() - timeWindowSeconds * 1000);
            
            xScale = d3.scaleTime()
                .domain([past, now])
                .range([0, width]);
            
            yScaleWaveform = d3.scaleLinear()
                .domain([-100, 100]) // Will be updated with actual data
                .range([height, 0]);
            
            // Create axes
            const xAxis = d3.axisBottom(xScale)
                .tickFormat(d3.timeFormat("%H:%M:%S"));
            
            const yAxis = d3.axisLeft(yScaleWaveform);
            
            // Add grid lines
            g.append("g")
                .attr("class", "grid")
                .attr("transform", "translate(0," + height + ")")
                .call(d3.axisBottom(xScale)
                    .tickSize(-height)
                    .tickFormat("")
                );
            
            g.append("g")
                .attr("class", "grid")
                .call(d3.axisLeft(yScaleWaveform)
                    .tickSize(-width)
                    .tickFormat("")
                );
            
            // Add axes
            g.append("g")
                .attr("class", "axis x-axis")
                .attr("transform", "translate(0," + height + ")")
                .call(xAxis);
            
            g.append("g")
                .attr("class", "axis y-axis")
                .call(yAxis);
            
            // Add axis labels
            g.append("text")
                .attr("class", "axis-label")
                .attr("transform", "rotate(-90)")
                .attr("y", 0 - margin.left)
                .attr("x", 0 - (height / 2))
                .attr("dy", "1em")
                .style("text-anchor", "middle")
                .text("Velocity (μm/s)");
            
            g.append("text")
                .attr("class", "axis-label")
                .attr("transform", "translate(" + (width / 2) + ", " + (height + margin.bottom) + ")")
                .style("text-anchor", "middle")
                .text("Time");
            
            // Add line path
            g.append("path")
                .attr("class", "waveform-path")
                .attr("d", "");
        }

        function initSpectrogramChart() {
            // Clear any existing SVG
            d3.select("#spectrogram-svg").selectAll("*").remove();
            
            // Create SVG
            spectrogramSvg = d3.select("#spectrogram-svg")
                .attr("width", width + margin.left + margin.right)
                .attr("height", height + margin.top + margin.bottom);
            
            const g = spectrogramSvg.append("g")
                .attr("transform", "translate(" + margin.left + "," + margin.top + ")");
            
            // Initialize scales (same x-scale as waveform for synchronization)
            const now = new Date();
            const past = new Date(now.getTime() - timeWindowSeconds * 1000);
            
            xScale = d3.scaleTime()
                .domain([past, now])
                .range([0, width]);
            
            yScaleSpectrogram = d3.scaleLinear()
                .domain([minFreq, maxFreq])
                .range([height, 0]);
            
            // Color scale for spectrogram
            colorScale = d3.scaleSequential(d3.interpolateViridis)
                .domain([0, 1]);
            
            // Create axes
            const xAxis = d3.axisBottom(xScale)
                .tickFormat(d3.timeFormat("%H:%M:%S"));
            
            const yAxis = d3.axisLeft(yScaleSpectrogram);
            
            // Add grid lines
            g.append("g")
                .attr("class", "grid")
                .attr("transform", "translate(0," + height + ")")
                .call(d3.axisBottom(xScale)
                    .tickSize(-height)
                    .tickFormat("")
                );
            
            g.append("g")
                .attr("class", "grid")
                .call(d3.axisLeft(yScaleSpectrogram)
                    .tickSize(-width)
                    .tickFormat("")
                );
            
            // Add axes
            g.append("g")
                .attr("class", "axis x-axis")
                .attr("transform", "translate(0," + height + ")")
                .call(xAxis);
            
            g.append("g")
                .attr("class", "axis y-axis")
                .call(yAxis);
            
            // Add axis labels
            g.append("text")
                .attr("class", "axis-label")
                .attr("transform", "rotate(-90)")
                .attr("y", 0 - margin.left)
                .attr("x", 0 - (height / 2))
                .attr("dy", "1em")
                .style("text-anchor", "middle")
                .text("Frequency (Hz)");
            
            g.append("text")
                .attr("class", "axis-label")
                .attr("transform", "translate(" + (width / 2) + ", " + (height + margin.bottom) + ")")
                .style("text-anchor", "middle")
                .text("Time");
            
            // Create group for spectrogram data
            g.append("g")
                .attr("class", "spectrogram-data");
            
            // Initialize empty spectrogram data
            spectrogramData = [];
        }
        
        function updateSpectrogramConfig() {
            if (yScaleSpectrogram) {
                yScaleSpectrogram.domain([minFreq, maxFreq]);
                // Update axes
                spectrogramSvg.select(".y-axis")
                    .call(d3.axisLeft(yScaleSpectrogram));
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
            if (!spectrogramSvg || !activeChannel || !channelData[activeChannel]) return;
            
            const channelInfo = channelData[activeChannel];
            const data = channelInfo.data;
            const times = channelInfo.times;
            
            if (data.length < spectrogramWindowSize || times.length === 0) return;
            
            // Calculate new spectrogram column
            const latestData = data.slice(-spectrogramWindowSize);
            const latestTime = times[times.length - 1];
            
            // Apply Hanning window
            const windowedData = latestData.map((val, idx) => {
                const hanningFactor = 0.5 * (1 - Math.cos(2 * Math.PI * idx / (spectrogramWindowSize - 1)));
                return val * hanningFactor;
            });
            
            // Compute FFT
            const fftResult = fft(windowedData);
            
            // Calculate power spectrum
            const freqResolution = sampleRate / spectrogramWindowSize;
            const totalFreqBins = Math.floor(spectrogramWindowSize / 2);
            
            // Create new spectrogram column data
            const newColumn = [];
            let minPower = Infinity;
            let maxPower = -Infinity;
            
            for (let k = 0; k < totalFreqBins; k++) {
                const frequency = k * freqResolution;
                
                if (frequency >= minFreq && frequency <= maxFreq) {
                    const magnitude = fftResult[k].real * fftResult[k].real + fftResult[k].imag * fftResult[k].imag;
                    const power = Math.log10(Math.max(magnitude / spectrogramWindowSize, 1e-10));
                    
                    newColumn.push({
                        time: latestTime,
                        frequency: frequency,
                        power: power
                    });
                    
                    minPower = Math.min(minPower, power);
                    maxPower = Math.max(maxPower, power);
                }
            }
            
            // Normalize power values
            newColumn.forEach(point => {
                point.normalizedPower = maxPower > minPower ? (point.power - minPower) / (maxPower - minPower) : 0;
            });
            
            // Add to spectrogram data
            spectrogramData.push({
                time: latestTime,
                data: newColumn
            });
            
            // Keep only recent data
            const timeThreshold = new Date(latestTime.getTime() - timeWindowSeconds * 1000);
            spectrogramData = spectrogramData.filter(col => col.time >= timeThreshold);
            
            // Render spectrogram
            renderSpectrogram();
        }
        
        function renderSpectrogram() {
            if (!spectrogramData.length) return;
            
            const g = spectrogramSvg.select("g").select(".spectrogram-data");
            
            // Update spectrogram x-axis to match waveform
            spectrogramSvg.select(".x-axis")
                .call(d3.axisBottom(xScale).tickFormat(d3.timeFormat("%H:%M:%S")));
            
            // Calculate rectangle dimensions
            const freqStep = (maxFreq - minFreq) / height; // Hz per pixel
            const timeStep = 1000; // 1 second per column
            
            // Create rectangles for each time column
            const columns = g.selectAll(".spectrogram-column")
                .data(spectrogramData, d => d.time);
            
            // Remove old columns
            columns.exit().remove();
            
            // Add new columns
            const newColumns = columns.enter()
                .append("g")
                .attr("class", "spectrogram-column");
            
            // Update all columns
            const allColumns = newColumns.merge(columns);
            
            allColumns.each(function(columnData) {
                const column = d3.select(this);
                
                // Create rectangles for frequency bins
                const rects = column.selectAll("rect")
                    .data(columnData.data);
                
                rects.enter()
                    .append("rect")
                    .merge(rects)
                    .attr("x", xScale(columnData.time))
                    .attr("y", function(d) { return yScaleSpectrogram(d.frequency + freqStep); })
                    .attr("width", Math.max(2, xScale(new Date(columnData.time.getTime() + timeStep)) - xScale(columnData.time)))
                    .attr("height", function(d) { return Math.max(1, yScaleSpectrogram(d.frequency) - yScaleSpectrogram(d.frequency + freqStep)); })
                    .attr("fill", function(d) { return colorScale(d.normalizedPower); });
                
                rects.exit().remove();
            });
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
            
            // Update spectrogram configuration
            updateSpectrogramConfig();
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
            
			let maxDataPoints = sampleRate * timeWindowSeconds;
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
            
            // Clear spectrogram data when switching channels
            spectrogramData = [];
            if (spectrogramSvg) {
                spectrogramSvg.select(".spectrogram-data").selectAll("*").remove();
            }
            
            updateChart();
        }

        function updateChart() {
            if (!waveformSvg || !activeChannel || !channelData[activeChannel]) return;
            
            const channelInfo = channelData[activeChannel];
            
            if (channelInfo.times.length === 0) return;
            
            // Calculate mean of current data
            const mean = channelInfo.data.length > 0 ? 
                channelInfo.data.reduce((sum, val) => sum + val, 0) / channelInfo.data.length : 0;
            
            // Create data points with mean-centered values converted to μm/s
            waveformData = [];
            for (let i = 0; i < channelInfo.times.length; i++) {
                waveformData.push({
                    time: channelInfo.times[i],
                    value: (channelInfo.data[i] - mean) / 1000 // Convert nm/s to μm/s
                });
            }
            
            // Update time scale
            if (waveformData.length > 0) {
                const latestTime = waveformData[waveformData.length - 1].time;
                const earliestTime = new Date(latestTime.getTime() - timeWindowSeconds * 1000);
                
                xScale.domain([earliestTime, latestTime]);
                
                // Update x-axis
                waveformSvg.select(".x-axis")
                    .call(d3.axisBottom(xScale).tickFormat(d3.timeFormat("%H:%M:%S")));
            }
            
            // Update y-scale based on data
            if (waveformData.length > 0) {
                const values = waveformData.map(d => d.value);
                const maxAbs = Math.max(...values.map(Math.abs));
                const padding = maxAbs * 0.1;
                
                yScaleWaveform.domain([-maxAbs - padding, maxAbs + padding]);
                
                // Update y-axis
                waveformSvg.select(".y-axis")
                    .call(d3.axisLeft(yScaleWaveform));
            }
            
            // Use all waveform data (don't filter by visible window)
            const visibleData = waveformData;
            
            // Create line generator
            const line = d3.line()
                .x(function(d) { return xScale(d.time); })
                .y(function(d) { return yScaleWaveform(d.value); })
                .curve(d3.curveLinear);
            
            // Update the path
            waveformSvg.select(".waveform-path")
                .datum(visibleData)
                .attr("d", line);
            
            // Update grid lines
            waveformSvg.select(".grid").selectAll("line")
                .data(xScale.ticks())
                .join("line")
                .attr("x1", function(d) { return xScale(d); })
                .attr("x2", function(d) { return xScale(d); })
                .attr("y1", 0)
                .attr("y2", height)
                .attr("class", "grid-line");
            
            // Always update spectrogram when waveform updates
            updateSpectrogramChart();
        }

        function formatUnits(data, originalUnits) {
            // Always return μm/s since we're converting from nm/s data
            return 'μm/s';
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
