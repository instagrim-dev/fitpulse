import { FormEvent, useEffect, useState } from 'react';
import { issueToken } from '../api/identity';
import { trackEvent } from '../telemetry/logger';

const DEFAULT_SCOPES = ['activities:write', 'activities:read', 'ontology:read'];

/** View-model properties used by `AuthPanel`. */
interface Props {
  /** Current tenant identifier propagated to downstream requests. */
  tenantId: string;
  /** Setter for the tenant identifier. */
  setTenantId: (tenant: string) => void;
  /** Active bearer token (if any). */
  token: string;
  /** Setter for the bearer token. */
  setToken: (token: string) => void;
  /** Callback invoked when a full token bundle (access + refresh) is received. */
  onTokenBundle: (bundle: TokenBundle) => void;
  /** Callback triggered when the user manually edits the token textarea, invalidating refresh state. */
  onManualTokenOverride: () => void;
  /** Account identifier used when requesting tokens. */
  accountId: string;
  /** Setter for the account identifier. */
  setAccountId: (id: string) => void;
  /** Whether the form should persist credentials in local storage. */
  remember: boolean;
  /** Toggle for persisting credentials. */
  setRemember: (remember: boolean) => void;
  /** Flag indicating the app is currently refreshing a token in the background. */
  refreshing?: boolean;
  /** Optional error shown when background refresh fails. */
  refreshError?: string | null;
}

export interface TokenBundle {
  accessToken: string;
  refreshToken: string;
  expiresIn: number;
  refreshExpiresIn: number;
  scopes: string[];
  source?: 'issue' | 'refresh' | 'restore';
}

type TokenStatus = 'idle' | 'pending' | 'success' | 'error';

/**
 * Authentication panel that can request JWTs from the identity service or accept manual tokens.
 * The component also exposes knobs for persisting credentials in local storage for convenient
 * operator workflows.
 */
export function AuthPanel({
  tenantId,
  setTenantId,
  token,
  setToken,
  onTokenBundle,
  onManualTokenOverride,
  accountId,
  setAccountId,
  remember,
  setRemember,
  refreshing = false,
  refreshError = null,
}: Props) {
  const [scopes, setScopes] = useState(DEFAULT_SCOPES.join(','));
  const [status, setStatus] = useState<TokenStatus>('idle');
  const [error, setError] = useState<string | null>(null);
  const [expiresIn, setExpiresIn] = useState<number | null>(null);

  useEffect(() => {
    setError(null);
    setStatus('idle');
    setExpiresIn(null);
  }, [accountId, tenantId]);

  const handleSubmit = async (evt: FormEvent) => {
    evt.preventDefault();
    setStatus('pending');
    setError(null);
    const scopesList = scopes
      .split(',')
      .map((s) => s.trim())
      .filter(Boolean);
    try {
      const result = await issueToken({
        account_id: accountId,
        tenant_id: tenantId,
        scopes: scopesList,
      });
      setToken(result.access_token);
      setExpiresIn(result.expires_in);
      setStatus('success');
      onTokenBundle({
        accessToken: result.access_token,
        refreshToken: result.refresh_token,
        expiresIn: result.expires_in,
        refreshExpiresIn: result.refresh_expires_in,
        scopes: scopesList,
        source: 'issue',
      });
      trackEvent('auth.token.issue.success', {
        tenantId,
        accountId,
        scopes: scopesList,
        expiresIn: result.expires_in,
        refreshExpiresIn: result.refresh_expires_in,
      });
    } catch (err) {
      setStatus('error');
      setError(err instanceof Error ? err.message : String(err));
      trackEvent('auth.token.issue.failure', {
        tenantId,
        accountId,
        error: err instanceof Error ? err.message : String(err),
      });
    }
  };

  return (
    <section className="panel">
      <h2>Authentication</h2>
      <form onSubmit={handleSubmit} className="auth-form">
        <label>
          Tenant ID
          <input
            value={tenantId}
            onChange={(e) => setTenantId(e.target.value)}
            required
            placeholder="tenant-123"
          />
        </label>
        <label>
          Account ID
          <input
            value={accountId}
            onChange={(e) => setAccountId(e.target.value)}
            required
            placeholder="account-uuid"
          />
        </label>
        <label>
          Scopes (comma separated)
          <input value={scopes} onChange={(e) => setScopes(e.target.value)} />
        </label>
        <div className="auth-actions">
          <button type="submit" disabled={status === 'pending'}>
            {status === 'pending' ? 'Requesting…' : 'Request Token'}
          </button>
          <label className="remember-toggle">
            <input
              type="checkbox"
              checked={remember}
              onChange={(e) => setRemember(e.target.checked)}
            />
            Remember token locally
          </label>
        </div>
      </form>

      <label>
        JWT Access Token
        <textarea
          value={token}
          onChange={(e) => {
            setToken(e.target.value);
            onManualTokenOverride();
            setExpiresIn(null);
            setStatus('idle');
          }}
          placeholder="Paste JWT issued by Identity Service"
          rows={5}
        />
      </label>
      {status === 'success' && token && (
        <p className="success">
          Token issued. Expires in approximately {Math.round((expiresIn ?? 0) / 60)} minutes.
        </p>
      )}
      {status === 'error' && <p className="error">Token request failed: {error}</p>}
      {refreshing && <p className="info">Refreshing token…</p>}
      <small>
        Tokens are issued via Identity Service `/v1/token`. Ensure the account exists and scopes
        include <code>activities:write</code>/<code>activities:read</code> and <code>ontology:read</code>.
      </small>
    </section>
  );
}
