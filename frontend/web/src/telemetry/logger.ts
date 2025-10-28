export interface TelemetryEvent {
  name: string;
  timestamp: string;
  properties?: Record<string, unknown>;
}

const endpoint = import.meta.env.VITE_TELEMETRY_URL ?? '';
const enabled = (import.meta.env.VITE_ENABLE_TELEMETRY ?? 'false').toLowerCase() === 'true';

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

export function trackEvent(name: string, properties?: Record<string, unknown>) {
  const event: TelemetryEvent = {
    name,
    timestamp: new Date().toISOString(),
    properties,
  };
  sendEvent(event);
}
