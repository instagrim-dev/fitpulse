import { render, screen } from '@testing-library/react';
import { QueryClient, QueryClientProvider } from '@tanstack/react-query';
import { vi } from 'vitest';
import * as ontologyApi from '../api/ontology';
import { AuthContext } from '../context/AuthContext';
import { OntologyInsights } from './OntologyInsights';

describe('OntologyInsights', () => {
  let queryClient: QueryClient;

  beforeEach(() => {
    queryClient = new QueryClient({ defaultOptions: { queries: { retry: false } } });
    vi.restoreAllMocks();
  });

  afterEach(() => {
    queryClient.clear();
  });

  function renderWithToken(token: string) {
    return render(
      <AuthContext.Provider value={{ token, tenantId: 'tenant' }}>
        <QueryClientProvider client={queryClient}>
          <OntologyInsights query="ride" />
        </QueryClientProvider>
      </AuthContext.Provider>
    );
  }

  it('renders nothing when no token', () => {
    renderWithToken('');
    expect(screen.queryByText('Ontology Insights')).toBeNull();
  });

  it('shows top exercises', async () => {
    vi.spyOn(ontologyApi, 'searchExercises').mockResolvedValue({
      items: [
        { id: 'ex1', name: 'Tempo Ride', difficulty: 'intermediate', targets: ['cardio'] },
      ],
    });

    renderWithToken('token');
    expect(await screen.findByText('Ontology Insights')).toBeInTheDocument();
    expect(await screen.findByText('Tempo Ride')).toBeInTheDocument();
  });
});
