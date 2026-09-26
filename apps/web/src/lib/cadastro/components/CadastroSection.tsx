import { ChevronDown } from "lucide-react";
import { type ReactNode, useState } from "react";

export interface CadastroSectionProps {
  icon: ReactNode;
  title: ReactNode;
  badge?: ReactNode | undefined;
  extraTitle?: ReactNode | undefined;
  hint?: string | undefined;
  defaultOpen?: boolean | undefined;
  open?: boolean | undefined;
  onToggleOpen?: (() => void) | undefined;
  modifier?: string | undefined;
  className?: string | undefined;
  children: ReactNode;
}

export function CadastroSectionBadge({
  modifier,
  children,
}: {
  modifier?: string | undefined;
  children: ReactNode;
}) {
  const extra = modifier ? ` cadastro-group__badge--${modifier}` : "";
  return <span className={`cadastro-group__badge${extra}`}>{children}</span>;
}

export function CadastroSection({
  icon,
  title,
  badge,
  extraTitle,
  hint,
  defaultOpen = true,
  open: controlledOpen,
  onToggleOpen: controlledToggle,
  modifier,
  className = "",
  children,
}: CadastroSectionProps) {
  const [uncontrolledOpen, setUncontrolledOpen] = useState(defaultOpen);
  const isOpen = controlledOpen !== undefined ? controlledOpen : uncontrolledOpen;
  const handleToggle = controlledToggle ?? (() => setUncontrolledOpen((prev) => !prev));

  const sectionModifier = modifier ? ` cadastro-group--${modifier}` : "";
  const iconModifier = modifier ? ` cadastro-group__icon--${modifier}` : "";

  return (
    <section
      className={`cadastro-group${sectionModifier} ${isOpen ? "" : "cadastro-group--collapsed"} ${className}`.trim()}
    >
      <header className="cadastro-group__header" onClick={handleToggle}>
        <div className="cadastro-group__title-area">
          <span className={`cadastro-group__icon${iconModifier}`}>{icon}</span>
          {modifier ? (
            <div className="cadastro-group__title-wrap">
              <h2 className="cadastro-group__title">{title}</h2>
              {badge}
              {extraTitle ? (
                <div className="cadastro-group__extra" onClick={(event) => event.stopPropagation()}>
                  {extraTitle}
                </div>
              ) : null}
            </div>
          ) : (
            <>
              <h2 className="cadastro-group__title">{title}</h2>
              {badge}
              {extraTitle ? (
                <div className="cadastro-group__extra" onClick={(event) => event.stopPropagation()}>
                  {extraTitle}
                </div>
              ) : null}
            </>
          )}
        </div>
        <ChevronDown className="cadastro-group__chevron" size={18} />
      </header>

      {isOpen && (
        <div className="cadastro-group__body">
          {hint && <p className="cadastro-group__hint">{hint}</p>}
          {children}
        </div>
      )}
    </section>
  );
}
