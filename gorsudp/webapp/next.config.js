/** @type {import('next').NextConfig} */
const nextConfig = {
  output: 'export',
  trailingSlash: true,
  images: {
    unoptimized: true
  },
  // Enable static export for integration with Go backend
  distDir: 'dist',
}

module.exports = nextConfig