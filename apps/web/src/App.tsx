import { useEffect, useState } from "react";
import { checkLiveHealth } from "./lib/api/health";

type HealthState = "checking" | "available" | "unavailable";

export function App() {
  const [health, setHealth] = useState<HealthState>("checking");

  useEffect(() => {
    const controller = new AbortController();

    void checkLiveHealth(controller.signal)
      .then((available) => {
        setHealth(available ? "available" : "unavailable");
      })
      .catch((error: unknown) => {
        if (error instanceof DOMException && error.name === "AbortError") {
          return;
        }
        setHealth("unavailable");
      });

    return () => controller.abort();
  }, []);

  return (
    <main className="page-shell">
      <section className="hero" aria-labelledby="page-title">
        <p className="eyebrow">Milestone 0</p>
        <h1 id="page-title">Gymkhana Database</h1>
        <p className="description">
          Fundação técnica do novo sistema. Os módulos de negócio serão adicionados em incrementos
          verticais.
        </p>
        <dl className="status-grid">
          <div>
            <dt>Frontend</dt>
            <dd>React + TypeScript</dd>
          </div>
          <div>
            <dt>API</dt>
            <dd data-health={health}>{healthLabel(health)}</dd>
          </div>
          <div>
            <dt>Contrato</dt>
            <dd>OpenAPI 3.1</dd>
          </div>
        </dl>
      </section>
    </main>
  );
}

function healthLabel(health: HealthState): string {
  switch (health) {
    case "available":
      return "Disponível";
    case "unavailable":
      return "Indisponível";
    default:
      return "Verificando";
  }
}
