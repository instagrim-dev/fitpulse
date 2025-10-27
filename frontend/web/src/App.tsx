import { useMemo, useState } from 'react';
import { ActivityForm } from './components/ActivityForm';
import { ActivityList } from './components/ActivityList';
import { OntologySearch } from './components/OntologySearch';
import { AuthContext } from './context/AuthContext';

const DEFAULT_USER_ID = import.meta.env.VITE_DEFAULT_USER_ID || 'user-1';

export default function App() {
  const [token, setToken] = useState('');
  const [tenantId, setTenantId] = useState('tenant-demo');
  const [userId, setUserId] = useState(DEFAULT_USER_ID);

  const authValue = useMemo(() => ({ token, tenantId }), [token, tenantId]);

  return (
    <AuthContext.Provider value={authValue}>
      <div className="container">
        <header>
          <h1>Fitness Activity Console</h1>
          <p>Interact with Activity and Exercise Ontology services.</p>
        </header>

        <section className="panel">
          <h2>Authentication</h2>
          <label>
            JWT Access Token
            <textarea
              value={token}
              onChange={(e) => setToken(e.target.value)}
              placeholder="Paste JWT issued by Identity Service"
            />
          </label>
          <label>
            Tenant ID
            <input value={tenantId} onChange={(e) => setTenantId(e.target.value)} />
          </label>
          <label>
            User ID
            <input value={userId} onChange={(e) => setUserId(e.target.value)} />
          </label>
          <small>
            Tokens can be generated via Identity Service `/v1/token`. Ensure scopes include
            <code>activities:write</code>/<code>activities:read</code> and <code>ontology:read</code>.
          </small>
        </section>

        <ActivityForm userId={userId} />
        <ActivityList userId={userId} />
        <OntologySearch />
      </div>
    </AuthContext.Provider>
  );
}
