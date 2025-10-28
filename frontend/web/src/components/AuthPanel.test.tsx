import { render, screen, waitFor } from '@testing-library/react';
import userEvent from '@testing-library/user-event';
import { AuthPanel } from './AuthPanel';

const issueTokenMock = vi.fn();

vi.mock('../api/identity', () => ({
  issueToken: (...args: unknown[]) => issueTokenMock(...args),
}));

vi.mock('../telemetry/logger', () => ({
  trackEvent: vi.fn(),
}));

describe('AuthPanel', () => {
  beforeEach(() => {
    issueTokenMock.mockResolvedValue({
      access_token: 'token123',
      token_type: 'bearer',
      expires_in: 3600,
      refresh_token: 'refresh123',
      refresh_expires_in: 7200,
      tenant_id: 'tenant-123',
    });
  });

  afterEach(() => {
    issueTokenMock.mockReset();
  });

  it('requests a token from the identity service', async () => {
    const user = userEvent.setup();
    const setToken = vi.fn();
    const setTenantId = vi.fn();
    const setAccountId = vi.fn();
    const setRemember = vi.fn();
    const onBundle = vi.fn();
    const onManualOverride = vi.fn();

    render(
      <AuthPanel
        token=""
        setToken={setToken}
        onTokenBundle={onBundle}
        onManualTokenOverride={onManualOverride}
        tenantId="tenant-123"
        setTenantId={setTenantId}
        accountId="account-456"
        setAccountId={setAccountId}
        remember
        setRemember={setRemember}
        refreshing={false}
        refreshError={null}
      />
    );

    await user.click(screen.getByRole('button', { name: /request token/i }));

    await waitFor(() => expect(issueTokenMock).toHaveBeenCalledTimes(1));
    expect(setToken).toHaveBeenCalledWith('token123');
    expect(onBundle).toHaveBeenCalledWith({
      accessToken: 'token123',
      refreshToken: 'refresh123',
      expiresIn: 3600,
      refreshExpiresIn: 7200,
      scopes: ['activities:write', 'activities:read', 'ontology:read'],
      source: 'issue',
    });
  });

  it('clears refresh metadata when token textarea changes', async () => {
    const user = userEvent.setup();
    const setToken = vi.fn();
    const onManualOverride = vi.fn();

    render(
      <AuthPanel
        token="manual"
        setToken={setToken}
        onTokenBundle={vi.fn()}
        onManualTokenOverride={onManualOverride}
        tenantId="tenant-123"
        setTenantId={vi.fn()}
        accountId="account-456"
        setAccountId={vi.fn()}
        remember
        setRemember={vi.fn()}
        refreshing={false}
        refreshError={null}
      />
    );

    const textarea = screen.getByPlaceholderText(/paste jwt/i);
    await user.type(textarea, 'x');

    expect(onManualOverride).toHaveBeenCalled();
  });
});
