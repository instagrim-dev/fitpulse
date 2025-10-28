/**
 * Envelope describing a telemetry event dispatched by the frontend.
 */
export interface TelemetryEvent {
  name: string;
  timestamp: string;
  properties?: Record<string, unknown>;
}

const endpoint = import.meta.env.VITE_TELEMETRY_URL ?? '';
const enabled = (import.meta.env.VITE_ENABLE_TELEMETRY ?? 'false').toLowerCase() === 'true';

/**
 * Dispatch the supplied telemetry event to the configured endpoint (or console in dev).
 *
 * @param event - Fully populated telemetry payload.
 */
function sendEvent(event: TelemetryEvent) {
  if (!enabled) {
    if (import.meta.env.DEV) {
      console.debug('[telemetry]', event);
    }
    return;
  }

  if (!endpoint) {
    console.warn('[telemetry] Missing VITE_TELEMETRY_URL; event dropped.', event);
    return;
  }

  const payload = JSON.stringify(event);
  try {
    if (typeof navigator !== 'undefined' && 'sendBeacon' in navigator) {
      const blob = new Blob([payload], { type: 'application/json' });
      navigator.sendBeacon(endpoint, blob);
      return;
    }
    void fetch(endpoint, {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      keepalive: true,
      body: payload,
    });
  } catch (err) {
    console.warn('[telemetry] Failed to send event', err, event);
  }
}

/**
 * Capture a telemetry event with optional structured properties. Events are routed to the
 * configured endpoint when telemetry is enabled, or logged to the console during development.
 *
 * @param name - Unique name describing the event (e.g., `auth.token.refresh.success`).
 * @param properties - Optional contextual metadata to include alongside the event.
 */
export function trackEvent(name: string, properties?: Record<string, unknown>) {
  const event: TelemetryEvent = {
    name,
    timestamp: new Date().toISOString(),
    properties,
  };
  sendEvent(event);
}
