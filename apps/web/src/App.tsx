import {
  Alert,
  AppShell,
  Button,
  Inline,
  Page,
  Stack,
  StatusBadge,
  Surface,
} from "@pherlsz/gymkhana-ui";
import { useCallback, useEffect, useState } from "react";
import { checkLiveHealth } from "./lib/api/health";

type HealthState = "checking" | "available" | "unavailable";

export function App() {
  const [health, setHealth] = useState<HealthState>("checking");

  const refreshHealth = useCallback(async (signal?: AbortSignal) => {
    setHealth("checking");

    try {
      const available = await checkLiveHealth(signal);
      setHealth(available ? "available" : "unavailable");
    } catch (error: unknown) {
      if (error instanceof DOMException && error.name === "AbortError") {
        return;
      }
      setHealth("unavailable");
    }
  }, []);

  useEffect(() => {
    const controller = new AbortController();
    void refreshHealth(controller.signal);
    return () => controller.abort();
  }, [refreshHealth]);

  return (
    <AppShell.Root>
      <AppShell.Header className="app-header">
        <strong>Gymkhana Database</strong>
        <StatusBadge tone="info">M1 foundations</StatusBadge>
      </AppShell.Header>
      <AppShell.Main>
        <Page.Root maxWidth="lg">
          <Page.Header>
            <Page.Eyebrow>Shared foundations</Page.Eyebrow>
            <Page.Title>Gymkhana Database</Page.Title>
            <Page.Description>
              Contratos de plataforma, componentes compartilhados e dependências versionadas para os
              primeiros incrementos verticais.
            </Page.Description>
            <Page.Actions>
              <Button
                aria-describedby="api-status"
                disabled={health === "checking"}
                onClick={() => void refreshHealth()}
              >
                Verificar API
              </Button>
            </Page.Actions>
          </Page.Header>

          <Page.Content>
            <Stack gap="6">
              {health === "unavailable" ? (
                <Alert title="API indisponível" tone="danger">
                  Verifique se o serviço local está em execução e tente novamente.
                </Alert>
              ) : null}

              <Page.Section
                description="Versões externas são consumidas somente por releases exatas."
                title="Foundation status"
              >
                <Inline align="stretch">
                  <FoundationCard label="Frontend" value="React + TypeScript" />
                  <FoundationCard label="Core" value="v0.2.1" />
                  <FoundationCard label="UI" value="v0.3.0" />
                  <Surface className="foundation-card" tone="raised">
                    <Stack gap="2">
                      <span className="foundation-card__label">API</span>
                      <StatusBadge id="api-status" tone={healthTone(health)}>
                        {healthLabel(health)}
                      </StatusBadge>
                    </Stack>
                  </Surface>
                </Inline>
              </Page.Section>
            </Stack>
          </Page.Content>
        </Page.Root>
      </AppShell.Main>
    </AppShell.Root>
  );
}

function FoundationCard({ label, value }: { label: string; value: string }) {
  return (
    <Surface className="foundation-card" tone="raised">
      <Stack gap="2">
        <span className="foundation-card__label">{label}</span>
        <strong>{value}</strong>
      </Stack>
    </Surface>
  );
}

function healthTone(health: HealthState): "neutral" | "success" | "danger" {
  switch (health) {
    case "available":
      return "success";
    case "unavailable":
      return "danger";
    default:
      return "neutral";
  }
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
