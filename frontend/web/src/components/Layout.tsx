import { ReactNode } from 'react';

/**
 * Props accepted by {@link Layout}.
 */
interface LayoutProps {
  /** Header region rendered at the top of the application shell. */
  header: ReactNode;
  /** Primary navigation/sidebar column. */
  sidebar: ReactNode;
  /** Main scrollable content region. */
  main: ReactNode;
}

/**
 * Provides the base application shell layout with header, sidebar, and main content slots.
 *
 * @param props - React nodes used to populate the three layout regions.
 */
export function Layout({ header, sidebar, main }: LayoutProps) {
  return (
    <div className="app-shell">
      <header className="app-shell__header">{header}</header>
      <div className="app-shell__body">
        <aside className="app-shell__sidebar">{sidebar}</aside>
        <main className="app-shell__main">{main}</main>
      </div>
    </div>
  );
}
