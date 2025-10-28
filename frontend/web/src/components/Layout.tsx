import { ReactNode } from 'react';

interface LayoutProps {
  header: ReactNode;
  sidebar: ReactNode;
  main: ReactNode;
}

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
