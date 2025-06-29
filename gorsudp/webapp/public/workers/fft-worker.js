// FFT Web Worker for spectrogram processing
// This worker handles computationally intensive FFT calculations off the main thread

// Enhanced FFT implementation with configurable zero-padding
function fft(signal, padTo = null) {
  const N = signal.length;
  if (N <= 1) return signal.map(x => ({ real: x, imag: 0 }));
  
  // Determine target size for zero-padding
  let targetSize;
  if (padTo && padTo > N) {
    // Use specified pad_to size (matches Python's pad_to parameter)
    targetSize = Math.pow(2, Math.ceil(Math.log2(padTo)));
  } else {
    // Default: ensure power of 2
    targetSize = Math.pow(2, Math.ceil(Math.log2(N)));
  }
  
  const paddedSignal = [...signal];
  while (paddedSignal.length < targetSize) {
    paddedSignal.push(0);
  }
  
  return fftRecursive(paddedSignal);
}

function fftRecursive(signal) {
  const N = signal.length;
  if (N <= 1) return signal.map(x => ({ real: x, imag: 0 }));
  
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

// Hanning window function
function hanningWindow(size) {
  const window = new Array(size);
  for (let i = 0; i < size; i++) {
    window[i] = 0.5 * (1 - Math.cos(2 * Math.PI * i / (size - 1)));
  }
  return window;
}

// Apply window function to signal
function applyWindow(signal, window) {
  if (signal.length !== window.length) {
    throw new Error('Signal and window must have the same length');
  }
  
  return signal.map((value, index) => value * window[index]);
}

// Calculate power spectrum from FFT result - Python compatible (sg**(1/10))
function calculatePowerSpectrum(fftResult, sampleRate, frequencyRange) {
  const [minFreq, maxFreq] = frequencyRange;
  const N = fftResult.length;
  const freqResolution = sampleRate / N;
  const maxBins = Math.floor(N / 2);
  
  const result = [];
  
  for (let k = 0; k < maxBins; k++) {
    const frequency = k * freqResolution;
    
    if (frequency >= minFreq && frequency <= maxFreq) {
      const magnitude = fftResult[k].real * fftResult[k].real + fftResult[k].imag * fftResult[k].imag;
      // Calculate power spectral density
      const psd = magnitude / (N * N);
      // Apply Python-style power scaling: 10th root (matches Python's sg**(1/10))
      // This provides more aggressive compression for better visual detail in low-power regions
      const power = Math.pow(psd, 1/10);
      
      result.push({
        frequency: frequency,
        power: power
      });
    }
  }
  
  return result;
}

// Normalize power values to 0-1 range
function normalizePowers(powers) {
  if (powers.length === 0) return powers;
  
  const powerValues = powers.map(p => p.power);
  const minPower = Math.min(...powerValues);
  const maxPower = Math.max(...powerValues);
  const range = maxPower - minPower;
  
  if (range === 0) {
    // When all powers are the same, return a moderate value (0.5) instead of 0
    // This prevents the spectrogram from being completely white
    return powers.map(p => ({ ...p, normalizedPower: 0.5 }));
  }
  
  return powers.map(p => ({
    ...p,
    normalizedPower: (p.power - minPower) / range
  }));
}

// Main message handler
self.onmessage = function(e) {
  try {
    const { samples, fftSize, sampleRate, frequencyRange } = e.data;
    
    // Validate input
    if (!Array.isArray(samples) || samples.length < fftSize) {
      throw new Error('Invalid input: samples must be an array with length >= fftSize');
    }
    
    // Take the last fftSize samples
    const inputSamples = samples.slice(-fftSize);
    
    // Apply Hanning window
    const window = hanningWindow(fftSize);
    const windowedSamples = applyWindow(inputSamples, window);
    
    // Compute FFT with zero-padding for better frequency resolution (matches Python's pad_to)
    const padTo = fftSize * 4;  // Match Python's pad_to=nfft*4 for interpolation
    const fftResult = fft(windowedSamples, padTo);
    
    // Calculate power spectrum
    const powerSpectrum = calculatePowerSpectrum(fftResult, sampleRate, frequencyRange);
    
    // Normalize powers
    const normalizedSpectrum = normalizePowers(powerSpectrum);
    
    // Debug logging for troubleshooting (can be removed once issue is resolved)
    if (powerSpectrum.length > 0) {
      const powers = powerSpectrum.map(p => p.power);
      const minPower = Math.min(...powers);
      const maxPower = Math.max(...powers);
      const normalizedPowers = normalizedSpectrum.map(p => p.normalizedPower);
      const minNormalized = Math.min(...normalizedPowers);
      const maxNormalized = Math.max(...normalizedPowers);
      
      // Only log if there might be an issue (low dynamic range)
      if (maxPower - minPower < 1e-10 || (minNormalized === maxNormalized && minNormalized === 0.5)) {
        console.debug('FFT Worker: Low dynamic range detected', {
          sampleCount: samples.length,
          powerRange: { min: minPower, max: maxPower, range: maxPower - minPower },
          normalizedRange: { min: minNormalized, max: maxNormalized },
          sampleRange: { min: Math.min(...samples), max: Math.max(...samples) }
        });
      }
    }
    
    // Prepare result
    const result = {
      timestamp: e.data.timestamp || Date.now(),
      frequencies: normalizedSpectrum.map(p => p.frequency),
      powers: normalizedSpectrum.map(p => p.power),
      normalizedPowers: normalizedSpectrum.map(p => p.normalizedPower),
      data: normalizedSpectrum
    };
    
    // Send result back to main thread
    self.postMessage(result);
    
  } catch (error) {
    // Send error back to main thread
    self.postMessage({
      error: error.message,
      timestamp: Date.now()
    });
  }
};