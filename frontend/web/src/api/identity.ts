const IDENTITY_API_URL = import.meta.env.VITE_IDENTITY_API_URL || 'http://localhost:8000';

export interface TokenRequestPayload {
  account_id: string;
  tenant_id: string;
  scopes?: string[];
}

export interface TokenResponse {
  access_token: string;
  token_type: string;
  expires_in: number;
  refresh_token: string;
  refresh_expires_in: number;
  tenant_id: string;
}

/**
 * Request an access/refresh token pair from the identity service for the given account.
 *
 * @throws Error when the identity service returns a non-2xx status.
 */
export async function issueToken(payload: TokenRequestPayload): Promise<TokenResponse> {
  const resp = await fetch(`${IDENTITY_API_URL}/v1/token`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(`Token request failed (${resp.status}): ${text || resp.statusText}`);
  }
  return (await resp.json()) as TokenResponse;
}

export interface RefreshTokenPayload {
  refresh_token: string;
  scopes?: string[];
}

/**
 * Exchange a refresh token for a new access/refresh pair, optionally overriding requested scopes.
 *
 * @param payload - Refresh token details, including the token string and optional scopes.
 * @returns A freshly minted {@link TokenResponse}.
 * @throws Error when the identity service returns a non-2xx status code.
 */
export async function refreshAccessToken(payload: RefreshTokenPayload): Promise<TokenResponse> {
  const resp = await fetch(`${IDENTITY_API_URL}/v1/token/refresh`, {
    method: 'POST',
    headers: {
      'Content-Type': 'application/json',
    },
    body: JSON.stringify(payload),
  });

  if (!resp.ok) {
    const text = await resp.text();
    throw new Error(`Refresh failed (${resp.status}): ${text || resp.statusText}`);
  }
  return (await resp.json()) as TokenResponse;
}
