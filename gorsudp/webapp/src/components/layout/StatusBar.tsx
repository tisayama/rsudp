'use client'

interface StatusBarProps {
  status: {
    active_channels: string[]
    client_count: number
    packets_received: number
    alerts_triggered: number
    uptime: string
  }
}

export const StatusBar: React.FC<StatusBarProps> = ({ status }) => {
  const formatNumber = (num: number) => {
    if (num >= 1000000) {
      return (num / 1000000).toFixed(1) + 'M'
    } else if (num >= 1000) {
      return (num / 1000).toFixed(1) + 'K'
    }
    return num.toString()
  }

  const formatUptime = (uptime: string) => {
    // Parse duration string and format it nicely
    if (uptime.includes('h')) {
      return uptime
    } else if (uptime.includes('m')) {
      return uptime
    } else if (uptime.includes('s')) {
      return uptime
    }
    return uptime
  }

  return (
    <div className="grid grid-cols-1 sm:grid-cols-2 lg:grid-cols-5 gap-4">
      <div className="status-item">
        <div className="status-label">System Status</div>
        <div className="status-value text-green-600">
          {status.active_channels.length > 0 ? 'Running' : 'Idle'}
        </div>
      </div>
      
      <div className="status-item">
        <div className="status-label">Active Channels</div>
        <div className="status-value">
          {status.active_channels.length > 0 
            ? status.active_channels.join(', ') 
            : 'None'
          }
        </div>
      </div>
      
      <div className="status-item">
        <div className="status-label">Connected Clients</div>
        <div className="status-value">
          {status.client_count}
        </div>
      </div>
      
      <div className="status-item">
        <div className="status-label">Packets Received</div>
        <div className="status-value">
          {formatNumber(status.packets_received)}
        </div>
      </div>
      
      <div className="status-item">
        <div className="status-label">Alerts Triggered</div>
        <div className="status-value text-red-600">
          {status.alerts_triggered}
        </div>
      </div>
    </div>
  )
}