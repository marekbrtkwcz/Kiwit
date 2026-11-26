import { useCallback, useEffect, useRef, useState } from 'react'
import { Bell } from 'lucide-react'
import { api, getAuthToken, NotificationItem } from '../../api/client'
import { useI18n } from '../../i18n'

function timeAgo(iso: string, labels: { justNow: string; minAgo: string; hourAgo: string; dayAgo: string }): string {
  const seconds = Math.max(0, (Date.now() - new Date(iso).getTime()) / 1000)
  if (seconds < 60) return labels.justNow
  const minutes = Math.floor(seconds / 60)
  if (minutes < 60) return `${minutes} ${labels.minAgo}`
  const hours = Math.floor(minutes / 60)
  if (hours < 24) return `${hours} ${labels.hourAgo}`
  return `${Math.floor(hours / 24)} ${labels.dayAgo}`
}

export function NotificationBell() {
  const { t } = useI18n()
  const [open, setOpen] = useState(false)
  const [items, setItems] = useState<NotificationItem[]>([])
  const [unread, setUnread] = useState(0)
  const [loading, setLoading] = useState(false)
  const rootRef = useRef<HTMLDivElement>(null)

  const refreshBadge = useCallback(async () => {
    if (!getAuthToken()) return
    try {
      const res = await api.listNotifications({ limit: 1 })
      setUnread(res.unread_count)
    } catch {
      /* badge polling is best-effort */
    }
  }, [])

  const loadList = useCallback(async () => {
    setLoading(true)
    try {
      const res = await api.listNotifications({ limit: 20 })
      setItems(res.items)
      setUnread(res.unread_count)
    } catch {
      /* leave previous items in place */
    } finally {
      setLoading(false)
    }
  }, [])

  useEffect(() => {
    void refreshBadge()
    const id = setInterval(() => void refreshBadge(), 30_000)
    return () => clearInterval(id)
  }, [refreshBadge])

  useEffect(() => {
    if (!open) return
    void loadList()
    const onDocMouseDown = (e: MouseEvent) => {
      if (rootRef.current && !rootRef.current.contains(e.target as Node)) {
        setOpen(false)
      }
    }
    document.addEventListener('mousedown', onDocMouseDown)
    return () => document.removeEventListener('mousedown', onDocMouseDown)
  }, [open, loadList])

  if (!getAuthToken()) return null

  const markRead = async (id: number) => {
    const before = items.find((n) => n.id === id)
    if (before?.read_at) return
    setItems((prev) =>
      prev.map((n) => (n.id === id ? { ...n, read_at: n.read_at ?? new Date().toISOString() } : n))
    )
    setUnread((u) => Math.max(0, u - 1))
    try {
      await api.readNotification(id)
    } catch {
      void loadList()
    }
  }

  const markAllRead = async () => {
    setItems((prev) => prev.map((n) => ({ ...n, read_at: n.read_at ?? new Date().toISOString() })))
    setUnread(0)
    try {
      await api.readAllNotifications()
    } catch {
      void loadList()
    }
  }

  return (
    <div className="notif-anchor" ref={rootRef}>
      <button
        type="button"
        className="btn btn-ghost btn-icon"
        onClick={() => setOpen((v) => !v)}
        title={t.notifications.title}
        aria-label={t.notifications.title}
        aria-expanded={open}
      >
        <Bell size={16} />
        {unread > 0 && <span className="notif-badge">{unread > 99 ? '99+' : unread}</span>}
      </button>

      {open && (
        <div className="notif-panel">
          <div className="notif-header">
            <span className="notif-title">{t.notifications.title}</span>
            <button type="button" className="btn btn-ghost" onClick={() => void markAllRead()}>
              {t.notifications.markAllRead}
            </button>
          </div>
          <div className="notif-list">
            {loading && items.length === 0 && <div className="notif-empty">{t.notifications.empty}</div>}
            {!loading && items.length === 0 && <div className="notif-empty">{t.notifications.empty}</div>}
            {items.map((n) => {
              const isUnread = !n.read_at
              return (
                <div
                  key={n.id}
                  className={`notif-item ${isUnread ? 'unread' : ''}`}
                  onClick={() => void markRead(n.id)}
                  role="button"
                  tabIndex={0}
                  onKeyDown={(e) => e.key === 'Enter' && void markRead(n.id)}
                >
                  <div className="notif-item-title">
                    {isUnread && <span className="notif-dot" aria-hidden />}
                    <span>{n.title}</span>
                  </div>
                  {n.body && <div className="notif-item-body">{n.body}</div>}
                  <div className="notif-item-time">
                    {timeAgo(n.created_at, {
                      justNow: t.notifications.justNow,
                      minAgo: t.notifications.minAgo,
                      hourAgo: t.notifications.hourAgo,
                      dayAgo: t.notifications.dayAgo,
                    })}
                  </div>
                </div>
              )
            })}
          </div>
        </div>
      )}
    </div>
  )
}
