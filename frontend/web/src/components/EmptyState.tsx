/**
 * Props accepted by {@link EmptyState}.
 */
interface EmptyStateProps {
  title: string;
  description?: string;
  variant?: 'neutral' | 'error';
  actionLabel?: string;
  onAction?: () => void;
  actionDisabled?: boolean;
}

/**
 * Presents a small empty/error panel with optional primary action for recovery flows.
 *
 * @param props - Content and behaviour configuration for the empty state.
 */
export function EmptyState({
  title,
  description,
  variant = 'neutral',
  actionLabel,
  onAction,
  actionDisabled,
}: EmptyStateProps) {
  return (
    <div className={`empty-state empty-state--${variant}`}>
      <h3 className="empty-state__title">{title}</h3>
      {description && <p className="empty-state__description">{description}</p>}
      {actionLabel && onAction && (
        <button
          type="button"
          className="empty-state__action"
          onClick={onAction}
          disabled={actionDisabled}
        >
          {actionLabel}
        </button>
      )}
    </div>
  );
}
