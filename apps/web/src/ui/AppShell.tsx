import React from "react";

interface AppShellProps {
  children?: React.ReactNode;
  [key: string]: any;
}

function AppShellRoot({ children, className, style, ...props }: AppShellProps) {
  return (
    <div className={className} style={{ display: "flex", minHeight: "100vh", backgroundColor: "#f9fafb", ...style }} {...props}>
      {children}
    </div>
  );
}

function AppShellHeader({ children, className, style, ...props }: AppShellProps) {
  return (
    <header className={className} style={{ backgroundColor: "white", borderBottom: "1px solid #e5e7eb", padding: "16px 24px", ...style }} {...props}>
      {children}
    </header>
  );
}

function AppShellMain({ children, className, style, ...props }: AppShellProps) {
  return (
    <main className={className} style={{ flex: 1, ...style }} {...props}>
      {children}
    </main>
  );
}

export const AppShell = Object.assign(AppShellRoot, {
  Root: AppShellRoot,
  Header: AppShellHeader,
  Main: AppShellMain,
});
