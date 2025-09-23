import { X } from 'lucide-react'
import { NotificationIcon, type Notification } from '@/hooks/useNotification'

interface NotificationContainerProps {
  notifications: Notification[]
  onRemove: (id: string) => void
}

export function NotificationContainer({ notifications, onRemove }: NotificationContainerProps) {
  if (notifications.length === 0) return null

  return (
    <div className="fixed top-4 right-4 z-50 flex flex-col gap-2 pointer-events-none">
      {notifications.map((notification) => (
        <div
          key={notification.id}
          className="pointer-events-auto animate-in slide-in-from-right fade-in duration-300"
        >
          <div className="bg-background border rounded-lg shadow-lg p-4 pr-10 min-w-[300px] max-w-[400px]">
            <button
              onClick={() => onRemove(notification.id)}
              className="absolute right-2 top-2 p-1 rounded-md hover:bg-muted transition-colors"
            >
              <X className="w-4 h-4" />
            </button>

            <div className="flex gap-3">
              <NotificationIcon type={notification.type} />
              <div className="flex-1">
                <p className="font-medium text-sm">{notification.title}</p>
                {notification.message && (
                  <p className="text-xs text-muted-foreground mt-1">{notification.message}</p>
                )}
              </div>
            </div>
          </div>
        </div>
      ))}
    </div>
  )
}