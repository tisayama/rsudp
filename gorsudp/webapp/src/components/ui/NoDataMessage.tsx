'use client'

interface NoDataMessageProps {
  message?: string
  icon?: string
  className?: string
}

export const NoDataMessage: React.FC<NoDataMessageProps> = ({
  message = 'No data available',
  icon = '📊',
  className = ''
}) => {
  return (
    <div className={`flex flex-col items-center justify-center p-8 text-center ${className}`}>
      <div className="text-4xl mb-4">{icon}</div>
      <h3 className="text-lg font-medium text-gray-900 mb-2">
        {message}
      </h3>
      <p className="text-gray-500 text-sm">
        Waiting for seismic data from the monitoring system...
      </p>
    </div>
  )
}