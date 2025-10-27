import { createContext } from 'react';

export interface AuthContextValue {
  token: string;
  tenantId: string;
}

export const AuthContext = createContext<AuthContextValue>({ token: '', tenantId: '' });
