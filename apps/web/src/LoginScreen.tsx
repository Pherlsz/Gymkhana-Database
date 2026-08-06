import { Button, Card, Typography } from "antd";
import { GoogleOutlined } from "@ant-design/icons";
import { useState } from "react";

const { Title, Text } = Typography;

export function LoginScreen({ onLogin }: { onLogin: () => void }) {
  const [loading, setLoading] = useState(false);

  const handleGoogleSignIn = async () => {
    setLoading(true);
    try {
      onLogin();
    } catch {
      setLoading(false);
    }
  };

  return (
    <div
      style={{
        minHeight: "100vh",
        display: "flex",
        alignItems: "center",
        justifyContent: "center",
        background: "linear-gradient(135deg, #667eea 0%, #764ba2 100%)",
        padding: "24px",
      }}
    >
      <Card
        style={{
          maxWidth: 400,
          width: "100%",
          borderRadius: 12,
          boxShadow: "0 8px 32px rgba(0, 0, 0, 0.1)",
        }}
      >
        <div style={{ textAlign: "center", marginBottom: 32 }}>
          <img
            src="/Gampa.png"
            alt="Gymkhana Database"
            style={{
              width: 120,
              height: 120,
              marginBottom: 16,
              objectFit: "contain",
            }}
          />
          <Title level={2} style={{ marginBottom: 8 }}>
            Gymkhana Database
          </Title>
          <Text>Faça login para continuar</Text>
        </div>

        <Button
          type="primary"
          size="large"
          icon={<GoogleOutlined />}
          loading={loading}
          onClick={handleGoogleSignIn}
          block
          style={{
            height: 48,
            fontSize: 16,
            fontWeight: 500,
          }}
        >
          {loading ? "Entrando..." : "Entrar com Google"}
        </Button>

        <div style={{ marginTop: 24, textAlign: "center" }}>
          <Text style={{ fontSize: 12 }}>
            Apenas usuários autorizados podem acessar
          </Text>
        </div>
      </Card>
    </div>
  );
}
