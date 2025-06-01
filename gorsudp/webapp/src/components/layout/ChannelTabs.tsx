'use client'

interface ChannelTabsProps {
  channels: string[]
  activeChannel: string
  onChannelChange: (channel: string) => void
}

export const ChannelTabs: React.FC<ChannelTabsProps> = ({
  channels,
  activeChannel,
  onChannelChange
}) => {
  if (channels.length === 0) {
    return null
  }

  return (
    <div className="bg-white border border-gray-200 rounded-lg p-4">
      <div className="flex items-center space-x-2 mb-2">
        <span className="text-sm font-medium text-gray-600">
          Active Channels:
        </span>
      </div>
      
      <div className="flex flex-wrap gap-1 max-h-32 overflow-y-auto">
        {channels.map((channel) => (
          <button
            key={channel}
            onClick={() => onChannelChange(channel)}
            className={`channel-tab ${
              activeChannel === channel ? 'active' : ''
            }`}
          >
            {channel}
          </button>
        ))}
      </div>
      
      {activeChannel && (
        <div className="mt-3 text-sm text-gray-600">
          Currently viewing: <span className="font-medium">{activeChannel}</span>
        </div>
      )}
    </div>
  )
}