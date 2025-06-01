'use client'

import type { AlertMessage } from '@/types'

interface AlertsPanelProps {
  alerts: AlertMessage['data'][]
}

export const AlertsPanel: React.FC<AlertsPanelProps> = ({ alerts }) => {
  if (alerts.length === 0) {
    return (
      <div className="bg-white border border-gray-200 rounded-lg p-4">
        <h3 className="text-lg font-semibold text-gray-800 mb-3">
          Earthquake Alerts
        </h3>
        <div className="text-gray-500 text-center py-8">
          No alerts yet. System is monitoring for seismic events.
        </div>
      </div>
    )
  }

  const formatTimestamp = (timestamp: string) => {
    return new Date(timestamp).toLocaleString()
  }

  const getAlertSeverity = (ratio: number) => {
    if (ratio >= 10) return { level: 'critical', color: 'border-red-500 bg-red-50' }
    if (ratio >= 5) return { level: 'high', color: 'border-orange-500 bg-orange-50' }
    if (ratio >= 2) return { level: 'medium', color: 'border-yellow-500 bg-yellow-50' }
    return { level: 'low', color: 'border-blue-500 bg-blue-50' }
  }

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4">
      <h3 className="text-lg font-semibold text-gray-800 mb-3">
        Earthquake Alerts ({alerts.length})
      </h3>
      
      <div className="space-y-3 max-h-96 overflow-y-auto">
        {alerts.map((alert, index) => {
          const severity = getAlertSeverity(alert.stalta_ratio)
          
          return (
            <div
              key={index}
              className={`p-3 border-l-4 rounded-r ${severity.color}`}
            >
              <div className="flex justify-between items-start">
                <div className="flex-1">
                  <div className="font-semibold text-gray-900">
                    EARTHQUAKE ALERT!
                  </div>
                  <div className="text-sm text-gray-700 mt-1">
                    <div>Channel: <span className="font-medium">{alert.channel}</span></div>
                    <div>STA/LTA Ratio: <span className="font-medium">{alert.stalta_ratio.toFixed(2)}</span></div>
                    {alert.message && (
                      <div>Message: <span className="font-medium">{alert.message}</span></div>
                    )}
                  </div>
                </div>
                <div className="text-xs text-gray-500 ml-4 whitespace-nowrap">
                  {formatTimestamp(alert.timestamp)}
                </div>
              </div>
              
              <div className="mt-2 flex items-center space-x-2">
                <span className={`inline-block px-2 py-1 text-xs font-medium rounded ${
                  severity.level === 'critical' ? 'bg-red-100 text-red-800' :
                  severity.level === 'high' ? 'bg-orange-100 text-orange-800' :
                  severity.level === 'medium' ? 'bg-yellow-100 text-yellow-800' :
                  'bg-blue-100 text-blue-800'
                }`}>
                  {severity.level.toUpperCase()}
                </span>
              </div>
            </div>
          )
        })}
      </div>
      
      {alerts.length >= 10 && (
        <div className="text-xs text-gray-500 text-center mt-3">
          Showing latest 10 alerts
        </div>
      )}
    </div>
  )
}