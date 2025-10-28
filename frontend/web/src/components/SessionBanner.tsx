interface SessionBannerProps {
  variant: 'warning' | 'error';
  message: string;
  primaryAction?: { label: string; onClick: () => void; disabled?: boolean };
  secondaryAction?: { label: string; onClick: () => void; disabled?: boolean };
}

export function SessionBanner({ variant, message, primaryAction, secondaryAction }: SessionBannerProps) {
  return (
    <div className={`session-banner session-banner--${variant}`} role={variant === 'error' ? 'alert' : 'status'}>
      <span className="session-banner__message">{message}</span>
      <div className="session-banner__actions">
        {primaryAction && (
          <button
            type="button"
            className="session-banner__button"
            onClick={primaryAction.onClick}
            disabled={primaryAction.disabled}
          >
            {primaryAction.label}
          </button>
        )}
        {secondaryAction && (
          <button
            type="button"
            className="session-banner__button session-banner__button--secondary"
            onClick={secondaryAction.onClick}
            disabled={secondaryAction.disabled}
          >
            {secondaryAction.label}
          </button>
        )}
      </div>
    </div>
  );
}
