import type { Metadata } from 'next'
import { ErrorBoundary } from '@/components/ui/ErrorBoundary'
import './globals.css'

export const metadata: Metadata = {
  title: 'GoRSUDP - Real-time Seismic Monitoring',
  description: 'Real-time seismic data visualization and monitoring system',
}

export default function RootLayout({
  children,
}: {
  children: React.ReactNode
}) {
  return (
    <html lang="en">
      <body className="min-h-screen bg-gray-50">
        <ErrorBoundary>
          {children}
        </ErrorBoundary>
      </body>
    </html>
  )
}