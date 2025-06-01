// FFT Web Worker for spectrogram processing
// This worker handles computationally intensive FFT calculations off the main thread

// Simple FFT implementation
function fft(signal) {
  const N = signal.length;
  if (N <= 1) return signal.map(x => ({ real: x, imag: 0 }));
  
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

// Calculate power spectrum from FFT result
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
      const power = Math.log10(Math.max(magnitude / N, 1e-10)); // Avoid log(0)
      
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
    return powers.map(p => ({ ...p, normalizedPower: 0 }));
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
    
    // Compute FFT
    const fftResult = fft(windowedSamples);
    
    // Calculate power spectrum
    const powerSpectrum = calculatePowerSpectrum(fftResult, sampleRate, frequencyRange);
    
    // Normalize powers
    const normalizedSpectrum = normalizePowers(powerSpectrum);
    
    // Prepare result
    const result = {
      timestamp: Date.now(),
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