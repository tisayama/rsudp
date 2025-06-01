/** @type {import('tailwindcss').Config} */
module.exports = {
  content: [
    './src/pages/**/*.{js,ts,jsx,tsx,mdx}',
    './src/components/**/*.{js,ts,jsx,tsx,mdx}',
    './src/app/**/*.{js,ts,jsx,tsx,mdx}',
  ],
  theme: {
    extend: {
      colors: {
        'seismic-blue': '#1f77b4',
        'seismic-orange': '#ff7f0e',
        'seismic-green': '#2ca02c',
        'seismic-red': '#d62728',
        'seismic-purple': '#9467bd',
        'seismic-brown': '#8c564b',
        'seismic-pink': '#e377c2',
        'seismic-gray': '#7f7f7f',
        'seismic-olive': '#bcbd22',
        'seismic-cyan': '#17becf',
      },
      fontFamily: {
        mono: ['Consolas', 'Monaco', 'Courier New', 'monospace'],
      },
      animation: {
        'pulse-slow': 'pulse 3s cubic-bezier(0.4, 0, 0.6, 1) infinite',
      },
    },
  },
  plugins: [],
}