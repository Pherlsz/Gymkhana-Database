import { CodeOutlined, GoogleOutlined } from "@ant-design/icons";
import { Alert, Button, Card, Divider, Flex, Image, Typography } from "antd";
import { useState } from "react";
import { useI18n } from "./i18n";
import { requestNoContent } from "./lib/api/client";
import "./login.css";

type LoginAction = "google" | "development" | null;

export function LoginScreen({ onLogin }: { onLogin: () => void }) {
  const { messages } = useI18n();
  const copy = messages.auth.login;
  const [loading, setLoading] = useState<LoginAction>(null);
  const [error, setError] = useState("");

  const handleGoogleSignIn = () => {
    setLoading("google");
    setError("");
    try {
      onLogin();
    } catch {
      setError(copy.googleStartError);
      setLoading(null);
    }
  };

  const handleDevelopmentSignIn = async () => {
    setLoading("development");
    setError("");
    try {
      await requestNoContent("/api/auth/dev-login", { method: "POST" });
      window.location.reload();
    } catch {
      setError(copy.developmentStartError);
      setLoading(null);
    }
  };

  return (
    <main className="login-page">
      <div aria-hidden className="login-page__background" />
      <div className="login-card-shell">
        <div aria-hidden className="login-card-shell__glow" />
        <Card
          className="login-card"
          styles={{ body: { padding: 0 } }}
          aria-labelledby="login-title"
        >
          <Flex className="login-card__header" vertical align="center" gap="small">
            <Image
              className="login-card__logo"
              src="/Gampa.png"
              alt={copy.logoAlt}
              preview={false}
            />
            <Flex vertical align="center" className="login-card__heading">
              <Typography.Title className="login-card__title" id="login-title" level={4}>
                {copy.title}
              </Typography.Title>
              <Typography.Text type="secondary" className="login-card__subtitle">
                {copy.subtitle}
              </Typography.Text>
            </Flex>
          </Flex>

          <Divider className="login-card__divider" />

          <Flex className="login-card__body" vertical gap="middle">
            {error ? <Alert message={error} type="error" showIcon /> : null}

            <Button
              block
              className="login-button"
              icon={<GoogleOutlined />}
              loading={loading === "google"}
              size="large"
              disabled={loading !== null && loading !== "google"}
              onClick={handleGoogleSignIn}
            >
              {copy.googleButton}
            </Button>

            {import.meta.env.DEV ? (
              <Button
                block
                className="login-button login-button--dev"
                icon={<CodeOutlined />}
                loading={loading === "development"}
                size="large"
                disabled={loading !== null && loading !== "development"}
                onClick={() => void handleDevelopmentSignIn()}
              >
                {copy.developmentButton}
              </Button>
            ) : null}
          </Flex>
        </Card>
      </div>
    </main>
  );
}
