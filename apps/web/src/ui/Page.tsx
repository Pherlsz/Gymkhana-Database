import React from "react";

interface PageProps {
  title?: string;
  children?: React.ReactNode;
  [key: string]: any;
}

function PageRoot({ children, className, style, ...props }: PageProps) {
  return (
    <div className={className} style={{ padding: "24px", maxWidth: "1400px", margin: "0 auto", ...style }} {...props}>
      {children}
    </div>
  );
}

function PageHeader({ children, className, style, ...props }: PageProps) {
  return (
    <div className={className} style={{ marginBottom: "24px", ...style }} {...props}>
      {children}
    </div>
  );
}

function PageEyebrow({ children, className, style, ...props }: PageProps) {
  return (
    <div className={className} style={{ fontSize: "14px", color: "#6b7280", marginBottom: "8px", ...style }} {...props}>
      {children}
    </div>
  );
}

function PageTitle({ children, className, style, ...props }: PageProps) {
  return (
    <h1 className={className} style={{ margin: 0, fontSize: "32px", fontWeight: 600, ...style }} {...props}>
      {children}
    </h1>
  );
}

function PageDescription({ children, className, style, ...props }: PageProps) {
  return (
    <p className={className} style={{ margin: "8px 0 0", fontSize: "14px", color: "#6b7280", ...style }} {...props}>
      {children}
    </p>
  );
}

function PageActions({ children, className, style, ...props }: PageProps) {
  return (
    <div className={className} style={{ marginTop: "16px", display: "flex", gap: "8px", ...style }} {...props}>
      {children}
    </div>
  );
}

function PageContent({ children, className, style, ...props }: PageProps) {
  return (
    <div className={className} style={style} {...props}>
      {children}
    </div>
  );
}

function PageSection({ children, className, style, ...props }: PageProps) {
  return (
    <section className={className} style={{ marginTop: "32px", ...style }} {...props}>
      {children}
    </section>
  );
}

export const Page = Object.assign(PageRoot, {
  Root: PageRoot,
  Header: PageHeader,
  Eyebrow: PageEyebrow,
  Title: PageTitle,
  Description: PageDescription,
  Actions: PageActions,
  Content: PageContent,
  Section: PageSection,
});
