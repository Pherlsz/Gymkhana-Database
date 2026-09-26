import { Button } from "antd";
import { useEffect, useState, type CSSProperties } from "react";
import { useI18n } from "./i18n";
import { apiURL } from "./lib/api/client";
import "./login.css";

type LoginAction = "google" | "development" | null;

const LOGIN_ORBS = [
  {
    w: 340,
    h: 340,
    top: "8%",
    left: "12%",
    blur: 80,
    opLight: 0.45,
    opDark: 0.13,
    delay: "0s",
    dur: "9s",
  },
  {
    w: 220,
    h: 220,
    top: "60%",
    left: "72%",
    blur: 60,
    opLight: 0.35,
    opDark: 0.09,
    delay: "2s",
    dur: "11s",
  },
] as const;

const SCATTERED_DOTS: [number, number][] = [
  [8, 12],
  [22, 55],
  [78, 8],
  [92, 38],
  [15, 80],
  [88, 72],
  [45, 92],
  [60, 18],
  [35, 45],
  [70, 60],
  [5, 50],
  [95, 20],
  [50, 5],
  [28, 70],
  [72, 85],
  [18, 28],
  [82, 50],
  [40, 30],
  [62, 76],
  [12, 95],
];

function GoogleIcon({ size = 20 }: { size?: number }) {
  return (
    <svg width={size} height={size} viewBox="0 0 24 24" aria-hidden>
      <path
        d="M22.56 12.25c0-.78-.07-1.53-.2-2.25H12v4.26h5.92c-.26 1.37-1.04 2.53-2.21 3.31v2.77h3.57c2.08-1.92 3.28-4.74 3.28-8.09z"
        fill="#4285F4"
      />
      <path
        d="M12 23c2.97 0 5.46-.98 7.28-2.66l-3.57-2.77c-.98.66-2.23 1.06-3.71 1.06-2.86 0-5.29-1.93-6.16-4.53H2.18v2.84C3.99 20.53 7.7 23 12 23z"
        fill="#34A853"
      />
      <path
        d="M5.84 14.09c-.22-.66-.35-1.36-.35-2.09s.13-1.43.35-2.09V7.07H2.18C1.43 8.55 1 10.22 1 12s.43 3.45 1.18 4.93l3.66-2.84z"
        fill="#FBBC05"
      />
      <path
        d="M12 5.38c1.62 0 3.06.56 4.21 1.64l3.15-3.15C17.45 2.09 14.97 1 12 1 7.7 1 3.99 3.47 2.18 7.07l3.66 2.84c.87-2.6 3.3-4.53 6.16-4.53z"
        fill="#EA4335"
      />
    </svg>
  );
}

export function LoginScreen({ onLogin }: { onLogin: () => void }) {
  return <LoginScreenContent onLogin={onLogin} />;
}

/** Shown while the SPA verifies the session cookie (F5 / OAuth return) — not the login form. */
export function AuthCheckingScreen() {
  const { messages } = useI18n();
  const copy = messages.auth.login;

  return (
    <div aria-busy="true" aria-live="polite" className="login-page">
      {LOGIN_ORBS.map((orb, index) => (
        <div
          key={index}
          aria-hidden
          className="login-orb"
          style={
            {
              width: orb.w,
              height: orb.h,
              top: orb.top,
              left: orb.left,
              filter: `blur(${orb.blur}px)`,
              animationDuration: orb.dur,
              animationDelay: orb.delay,
              "--orb-op-light": orb.opLight,
              "--orb-op-dark": orb.opDark,
            } as CSSProperties
          }
        />
      ))}
      <div aria-hidden className="login-vignette" />
      <main className="login-main">
        <div className="login-stack">
          <div className="login-mascot-enter">
            <div aria-hidden className="login-glow-ring" />
            <img alt="" className="login-mascot-img" height={220} src="/Gampa.png" width={220} />
          </div>
          <p className="login-checking">
            <span aria-hidden className="login-button__spinner" />
            <span>{copy.signingIn}</span>
          </p>
        </div>
      </main>
    </div>
  );
}

function LoginScreenContent({ onLogin }: { onLogin: () => void }) {
  const { messages } = useI18n();
  const copy = messages.auth.login;
  const [loading, setLoading] = useState<LoginAction>(null);
  const [error, setError] = useState("");

  useEffect(() => {
    const oauthError = new URLSearchParams(window.location.search).get("error");
    if (!oauthError) return;
    setError(oauthError === "AccessDenied" ? copy.accessDenied : copy.oauthError);
  }, [copy.accessDenied, copy.oauthError]);

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

  const handleDevelopmentSignIn = () => {
    setLoading("development");
    setError("");
    try {
      window.location.assign(apiURL("/api/auth/dev-login"));
    } catch {
      setError(copy.developmentStartError);
      setLoading(null);
    }
  };

  return (
    <div className="login-page">
      {LOGIN_ORBS.map((orb, index) => (
        <div
          key={index}
          className="login-orb"
          style={
            {
              width: orb.w,
              height: orb.h,
              top: orb.top,
              left: orb.left,
              filter: `blur(${orb.blur}px)`,
              animationDuration: orb.dur,
              animationDelay: orb.delay,
              "--orb-op-light": orb.opLight,
              "--orb-op-dark": orb.opDark,
            } as CSSProperties
          }
          aria-hidden
        />
      ))}

      <svg className="login-dots" aria-hidden>
        {SCATTERED_DOTS.map(([cx, cy], index) => (
          <circle
            key={index}
            cx={`${cx}%`}
            cy={`${cy}%`}
            r={index % 3 === 0 ? 2.5 : index % 3 === 1 ? 1.5 : 1}
          />
        ))}
      </svg>

      <div className="login-vignette" aria-hidden />

      <main className="login-main">
        <h1 className="visually-hidden">{copy.title}</h1>
        <div className="login-stack">
          <div className="login-mascot-enter">
            <div className="login-glow-ring" aria-hidden />
            <img className="login-mascot-img" src="/Gampa.png" alt="" width={220} height={220} />
          </div>

          <section className="login-card login-card-enter" aria-label={copy.title}>
            <div className="login-card-highlight" aria-hidden />
            <div className="login-card__body">
              {error ? (
                <p className="login-alert" role="alert">
                  {error}
                </p>
              ) : null}

              <div className="login-card-divider" aria-hidden />

              <Button
                className="login-google-btn"
                disabled={loading !== null}
                icon={loading !== "google" ? <GoogleIcon /> : undefined}
                loading={loading === "google"}
                onClick={handleGoogleSignIn}
                size="large"
              >
                {copy.googleButton}
              </Button>

              {import.meta.env.DEV ? (
                <Button
                  className="login-google-btn login-google-btn--dev"
                  disabled={loading !== null}
                  loading={loading === "development"}
                  onClick={handleDevelopmentSignIn}
                  size="large"
                >
                  {copy.developmentButton}
                </Button>
              ) : null}
            </div>
          </section>
        </div>
      </main>
    </div>
  );
}
