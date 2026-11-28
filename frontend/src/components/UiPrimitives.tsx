import { AlertTriangle, X } from 'lucide-react'
import { type ReactNode, useEffect, useRef } from 'react'
import { useI18n } from '../i18n'

interface ConfirmDialogProps {
  open: boolean
  title: string
  message: string
  confirmLabel?: string
  cancelLabel?: string
  danger?: boolean
  onConfirm: () => void
  onCancel: () => void
}

// ConfirmDialog — modal replacement for window.confirm, with focus trap and Escape handling.
export function ConfirmDialog({
  open,
  title,
  message,
  confirmLabel = 'Confirm',
  cancelLabel = 'Cancel',
  danger = true,
  onConfirm,
  onCancel,
}: ConfirmDialogProps) {
  const { t } = useI18n()
  const dialogRef = useRef<HTMLDivElement>(null)

  // Focus trap: keep Tab cycling inside the dialog, focus first button on open
  useEffect(() => {
    if (!open) return
    const el = dialogRef.current
    if (!el) return
    const focusable = el.querySelectorAll<HTMLElement>(
      'button, [href], input, select, textarea, [tabindex]:not([tabindex="-1"])',
    )
    focusable[0]?.focus()
    const onKeyDown = (e: KeyboardEvent) => {
      if (e.key !== 'Tab') return
      const first = focusable[0]
      const last = focusable[focusable.length - 1]
      if (e.shiftKey && document.activeElement === first) {
        e.preventDefault()
        last?.focus()
      } else if (!e.shiftKey && document.activeElement === last) {
        e.preventDefault()
        first?.focus()
      }
    }
    el.addEventListener('keydown', onKeyDown)
    return () => el.removeEventListener('keydown', onKeyDown)
  }, [open])

  if (!open) return null

  return (
    <div
      className="drawer-backdrop"
      style={{ alignItems: 'center', justifyContent: 'center' }}
      onClick={onCancel}
      role="presentation"
    >
      <div
        ref={dialogRef}
        className="card"
        style={{ maxWidth: 420, width: '90%', padding: '1.25rem' }}
        role="alertdialog"
        aria-modal="true"
        aria-label={title}
        onClick={(e) => e.stopPropagation()}
        onKeyDown={(e) => {
          if (e.key === 'Escape') onCancel()
          if (e.key === 'Enter') onConfirm()
        }}
      >
        <div style={{ display: 'flex', alignItems: 'flex-start', gap: '0.75rem' }}>
          {danger && (
            <span style={{ color: 'var(--danger, #dc2626)', flexShrink: 0, marginTop: 2 }}>
              <AlertTriangle size={20} />
            </span>
          )}
          <div style={{ flex: 1 }}>
            <h3 style={{ margin: '0 0 0.4rem', fontSize: '1rem' }}>{title}</h3>
            <p style={{ margin: 0, color: 'var(--text-secondary)', fontSize: '0.875rem' }}>{message}</p>
          </div>
          <button type="button" className="btn btn-ghost btn-icon" onClick={onCancel} aria-label={t.common.close}>
            <X size={16} />
          </button>
        </div>
        <div style={{ display: 'flex', gap: '0.5rem', justifyContent: 'flex-end', marginTop: '1.25rem' }}>
          <button type="button" className="btn" onClick={onCancel} autoFocus>
            {cancelLabel}
          </button>
          <button
            type="button"
            className="btn"
            style={
              danger
                ? { background: 'var(--danger, #dc2626)', color: '#fff', borderColor: 'transparent' }
                : { background: 'var(--primary)', color: '#fff', borderColor: 'transparent' }
            }
            onClick={onConfirm}
          >
            {confirmLabel}
          </button>
        </div>
      </div>
    </div>
  )
}

interface EmptyStateProps {
  icon?: ReactNode
  title: string
  description?: string
  actionLabel?: string
  onAction?: () => void
}

// EmptyState — friendly placeholder with CTA when a list is empty.
export function EmptyState({ icon, title, description, actionLabel, onAction }: EmptyStateProps) {
  return (
    <div
      style={{
        textAlign: 'center',
        padding: '3rem 1.5rem',
        color: 'var(--muted)',
        border: '1px dashed var(--border-strong)',
        borderRadius: 12,
      }}
    >
      {icon && <div style={{ marginBottom: '0.75rem', opacity: 0.6 }}>{icon}</div>}
      <div style={{ fontWeight: 600, color: 'var(--text)', marginBottom: '0.25rem' }}>{title}</div>
      {description && <div style={{ fontSize: '0.875rem', marginBottom: '1rem' }}>{description}</div>}
      {actionLabel && onAction && (
        <button type="button" className="btn btn-primary" onClick={onAction}>
          {actionLabel}
        </button>
      )}
    </div>
  )
}

// LoadingSkeleton — simple shimmer placeholder for list loading.
export function LoadingSkeleton({ rows = 5 }: { rows?: number }) {
  const { t } = useI18n()
  return (
    <div
      role="status"
      style={{ display: 'flex', flexDirection: 'column', gap: '0.5rem' }}
      aria-label={t.common.loading}
    >
      {Array.from({ length: rows }).map((_, i) => (
        <div
          key={i}
          style={{
            height: 40,
            borderRadius: 8,
            background: 'linear-gradient(90deg, var(--surface-2) 25%, var(--border) 50%, var(--surface-2) 75%)',
            backgroundSize: '200% 100%',
            animation: 'kiwit-shimmer 1.2s infinite',
          }}
        />
      ))}
    </div>
  )
}
