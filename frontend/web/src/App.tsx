import { useCallback, useEffect, useMemo, useState } from 'react';
import { ActivityForm } from './components/ActivityForm';
import { ActivityList } from './components/ActivityList';
import { ActivityDashboard } from './components/ActivityDashboard';
import { OntologySearch } from './components/OntologySearch';
import { OntologyInsights } from './components/OntologyInsights';
import { Layout } from './components/Layout';
import { AuthPanel } from './components/AuthPanel';
import { SessionBanner } from './components/SessionBanner';
import type { TokenBundle } from './components/AuthPanel';
import { AuthContext } from './context/AuthContext';
import { refreshAccessToken } from './api/identity';
import { trackEvent } from './telemetry/logger';

const DEFAULT_USER_ID = import.meta.env.VITE_DEFAULT_USER_ID || '11111111-1111-1111-1111-111111111111';
const STORAGE_KEY = 'i5e-auth';

/**
 * Root application shell combining auth controls with activity and ontology panels.
 * Maintains tenant and token state that downstream components consume via context.
 *
 * @returns The fully composed application layout.
 */
export default function App() {
  const [token, setToken] = useState('');
  const [tenantId, setTenantId] = useState('22222222-2222-2222-2222-222222222222');
  const [userId, setUserId] = useState(DEFAULT_USER_ID);
  const [accountId, setAccountId] = useState('11111111-1111-1111-1111-111111111111');
  const [remember, setRemember] = useState(true);
  const [ontologyQuery, setOntologyQuery] = useState('ride');
  const [refreshToken, setRefreshToken] = useState('');
  const [accessExpiresAt, setAccessExpiresAt] = useState<number | null>(null);
  const [refreshExpiresAt, setRefreshExpiresAt] = useState<number | null>(null);
  const [tokenScopes, setTokenScopes] = useState<string[]>([]);
  const [refreshing, setRefreshing] = useState(false);
  const [refreshError, setRefreshError] = useState<string | null>(null);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    const stored = window.localStorage.getItem(STORAGE_KEY);
    if (stored) {
      try {
        const parsed = JSON.parse(stored) as {
          token?: string;
          tenantId?: string;
          accountId?: string;
          userId?: string;
          remember?: boolean;
          ontologyQuery?: string;
          refreshToken?: string;
          accessExpiresAt?: number;
          refreshExpiresAt?: number;
          scopes?: string[];
        };
        if (parsed.token) setToken(parsed.token);
        if (parsed.tenantId) setTenantId(parsed.tenantId);
        if (parsed.accountId) setAccountId(parsed.accountId);
        if (parsed.userId) setUserId(parsed.userId);
        if (typeof parsed.remember === 'boolean') setRemember(parsed.remember);
        if (parsed.ontologyQuery) setOntologyQuery(parsed.ontologyQuery);
        if (parsed.refreshToken) setRefreshToken(parsed.refreshToken);
        if (typeof parsed.accessExpiresAt === 'number') setAccessExpiresAt(parsed.accessExpiresAt);
        if (typeof parsed.refreshExpiresAt === 'number') setRefreshExpiresAt(parsed.refreshExpiresAt);
        if (Array.isArray(parsed.scopes)) setTokenScopes(parsed.scopes);
      } catch {
        // ignore malformed storage
      }
    }
  }, []);

  useEffect(() => {
    if (typeof window === 'undefined') return;
    if (!remember) {
      window.localStorage.removeItem(STORAGE_KEY);
      return;
    }
    const payload = JSON.stringify({
      token,
      refreshToken,
      accessExpiresAt,
      refreshExpiresAt,
      scopes: tokenScopes,
      tenantId,
      accountId,
      userId,
      remember: true,
      ontologyQuery,
    });
    window.localStorage.setItem(STORAGE_KEY, payload);
  }, [token, refreshToken, accessExpiresAt, refreshExpiresAt, tokenScopes, tenantId, accountId, userId, remember, ontologyQuery]);

  const resetRefreshState = useCallback(() => {
    setRefreshToken('');
    setAccessExpiresAt(null);
    setRefreshExpiresAt(null);
    setTokenScopes([]);
  }, []);

  const handleManualTokenOverride = useCallback(() => {
    trackEvent('auth.token.manual_override', { hadToken: Boolean(token) });
    resetRefreshState();
    setRefreshError(null);
  }, [resetRefreshState, token]);

  const applyTokenBundle = useCallback((bundle: TokenBundle) => {
    const now = Date.now();
    setToken(bundle.accessToken);
    setRefreshToken(bundle.refreshToken);
    setAccessExpiresAt(now + bundle.expiresIn * 1000);
    setRefreshExpiresAt(now + bundle.refreshExpiresIn * 1000);
    setTokenScopes(bundle.scopes);
    setRefreshError(null);
    trackEvent('auth.token.bundle.applied', {
      source: bundle.source ?? 'unknown',
      expiresIn: bundle.expiresIn,
      refreshExpiresIn: bundle.refreshExpiresIn,
      scopes: bundle.scopes,
    });
  }, []);

  const clearSession = useCallback((message?: string) => {
    const hadToken = Boolean(token);
    setToken('');
    resetRefreshState();
    setRefreshError(message ?? null);
    trackEvent('auth.session.cleared', {
      reason: message ?? 'manual',
      hadToken,
    });
  }, [resetRefreshState, token]);

  const triggerRefresh = useCallback(async () => {
    if (!refreshToken) return;
    const startedAt = performance.now();
    try {
      setRefreshing(true);
      const response = await refreshAccessToken({
        refresh_token: refreshToken,
        scopes: tokenScopes.length ? tokenScopes : undefined,
      });
      applyTokenBundle({
        accessToken: response.access_token,
        refreshToken: response.refresh_token,
        expiresIn: response.expires_in,
        refreshExpiresIn: response.refresh_expires_in,
        scopes: tokenScopes,
        source: 'refresh',
      });
      trackEvent('auth.token.refresh.success', {
        durationMs: Math.round(performance.now() - startedAt),
        scopes: tokenScopes,
      });
    } catch (err) {
      console.warn('Token refresh failed', err);
      clearSession('Session expired. Please reauthenticate.');
      trackEvent('auth.token.refresh.failure', {
        error: err instanceof Error ? err.message : String(err),
      });
    } finally {
      setRefreshing(false);
    }
  }, [refreshToken, tokenScopes, applyTokenBundle, clearSession]);

  const handleManualRefresh = useCallback(() => {
    if (refreshing) return;
    trackEvent('auth.token.refresh.manual_request');
    void triggerRefresh();
  }, [refreshing, triggerRefresh]);

  const handleReauthenticate = useCallback(() => {
    clearSession();
  }, [clearSession]);

  useEffect(() => {
    if (!refreshToken || !accessExpiresAt) return;
    const margin = 60_000; // 1 minute
    const now = Date.now();
    const delay = accessExpiresAt - margin - now;
    if (delay <= 0) {
      triggerRefresh();
      return;
    }
    const timer = window.setTimeout(() => {
      triggerRefresh();
    }, delay);
    return () => window.clearTimeout(timer);
  }, [refreshToken, accessExpiresAt, triggerRefresh]);

  useEffect(() => {
    if (!refreshExpiresAt) return;
    const now = Date.now();
    if (refreshExpiresAt <= now) {
      clearSession('Session expired. Please reauthenticate.');
      return;
    }
    const timer = window.setTimeout(() => {
      clearSession('Session expired. Please reauthenticate.');
    }, refreshExpiresAt - now);
    return () => window.clearTimeout(timer);
  }, [refreshExpiresAt, clearSession]);

  const authValue = useMemo(() => ({ token, tenantId }), [token, tenantId]);

  const navLinks = [
    { href: '#overview', label: 'Overview' },
    { href: '#log', label: 'Log Activity' },
    { href: '#history', label: 'History' },
    { href: '#ontology', label: 'Ontology' },
  ];

  const timeRemaining = accessExpiresAt ? accessExpiresAt - Date.now() : null;
  const sessionExpiringSoon = Boolean(
    token && refreshToken && !refreshError && timeRemaining !== null && timeRemaining > 0 && timeRemaining <= 120_000
  );
  const sessionBanner = (() => {
    if (refreshError) {
      return (
        <SessionBanner
          variant="error"
          message={refreshError}
          primaryAction={{ label: 'Reauthenticate', onClick: handleReauthenticate }}
        />
      );
    }
    if (sessionExpiringSoon) {
      const seconds = Math.max(0, Math.round((timeRemaining ?? 0) / 1000));
      const display = seconds >= 60 ? `${Math.max(1, Math.round(seconds / 60))} min` : `${seconds} sec`;
      return (
        <SessionBanner
          variant="warning"
          message={`Session expires in ${display}.`}
          primaryAction={{
            label: refreshing ? 'Refreshing…' : 'Refresh now',
            onClick: handleManualRefresh,
            disabled: refreshing,
          }}
        />
      );
    }
    return null;
  })();

  const header = (
    <div className="app-header">
      <div className="app-header__copy">
        <h1>Fitness Activity</h1>
        <p>Monitor pipeline health, reconcile pending events, and explore ontology insights.</p>
      </div>
      <nav className="app-nav" aria-label="Primary">
        {navLinks.map((link) => (
          <a key={link.href} href={link.href}>
            {link.label}
          </a>
        ))}
      </nav>
    </div>
  );

  const sidebar = (
    <div className="sidebar">
      {sessionBanner}
      <AuthPanel
        token={token}
        setToken={setToken}
        onTokenBundle={applyTokenBundle}
        onManualTokenOverride={handleManualTokenOverride}
        tenantId={tenantId}
        setTenantId={setTenantId}
        accountId={accountId}
        setAccountId={(id) => {
          setAccountId(id);
          setUserId((prev) => (prev === DEFAULT_USER_ID ? id : prev));
        }}
        remember={remember}
        setRemember={setRemember}
        refreshing={refreshing}
        refreshError={refreshError}
      />
      <section className="panel">
        <h2>User Context</h2>
        <label>
          Active User ID
          <input
            value={userId}
            onChange={(e) => setUserId(e.target.value)}
            placeholder="user-123"
          />
        </label>
      </section>
      <OntologyInsights query={ontologyQuery} />
    </div>
  );

  const main = (
    <div className="main-content">
      <div id="overview" className="main-section">
        <ActivityDashboard userId={userId} />
      </div>
      <div id="log" className="main-section">
        <ActivityForm userId={userId} />
      </div>
      <div id="history" className="main-section">
        <ActivityList userId={userId} />
      </div>
      <div id="ontology" className="main-section">
        <OntologySearch query={ontologyQuery} onQueryChange={setOntologyQuery} />
      </div>
    </div>
  );

  return (
    <AuthContext.Provider value={authValue}>
      <Layout header={header} sidebar={sidebar} main={main} />
    </AuthContext.Provider>
  );
}
