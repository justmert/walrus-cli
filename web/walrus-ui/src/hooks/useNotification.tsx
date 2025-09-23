import { useState, useCallback } from 'react'
import { CheckCircle2, XCircle, AlertCircle, Info } from 'lucide-react'

export type NotificationType = 'success' | 'error' | 'warning' | 'info'

export interface Notification {
  id: string
  type: NotificationType
  title: string
  message?: string
}

export function useNotification() {
  const [notifications, setNotifications] = useState<Notification[]>([])

  const showNotification = useCallback((type: NotificationType, title: string, message?: string) => {
    const id = Date.now().toString()
    const notification: Notification = { id, type, title, message }

    setNotifications(prev => [...prev, notification])

    // Auto remove after 5 seconds
    setTimeout(() => {
      setNotifications(prev => prev.filter(n => n.id !== id))
    }, 5000)
  }, [])

  const removeNotification = useCallback((id: string) => {
    setNotifications(prev => prev.filter(n => n.id !== id))
  }, [])

  const success = useCallback((title: string, message?: string) => {
    showNotification('success', title, message)
  }, [showNotification])

  const error = useCallback((title: string, message?: string) => {
    showNotification('error', title, message)
  }, [showNotification])

  const warning = useCallback((title: string, message?: string) => {
    showNotification('warning', title, message)
  }, [showNotification])

  const info = useCallback((title: string, message?: string) => {
    showNotification('info', title, message)
  }, [showNotification])

  return {
    notifications,
    showNotification,
    removeNotification,
    success,
    error,
    warning,
    info
  }
}

export function NotificationIcon({ type }: { type: NotificationType }) {
  switch (type) {
    case 'success':
      return <CheckCircle2 className="w-5 h-5 text-green-500" />
    case 'error':
      return <XCircle className="w-5 h-5 text-red-500" />
    case 'warning':
      return <AlertCircle className="w-5 h-5 text-yellow-500" />
    case 'info':
      return <Info className="w-5 h-5 text-blue-500" />
  }
}