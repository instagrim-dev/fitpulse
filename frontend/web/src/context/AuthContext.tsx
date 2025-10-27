import { createContext } from 'react';

/**
 * Reactive authentication payload injected into components that need the current
 * JWT and tenant identifier.
 */
export interface AuthContextValue {
  token: string;
  tenantId: string;
}

/**
 * Application-wide context for authentication state. Consumers should expect an
 * empty token when the user has not provided credentials yet.
 */
export const AuthContext = createContext<AuthContextValue>({ token: '', tenantId: '' });
