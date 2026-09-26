import { Container, getRandom } from "@cloudflare/containers";

const INSTANCE_COUNT = 1;

type ApiEnv = {
  API_CONTAINER: DurableObjectNamespace<ApiContainer>;
  DATABASE_URL: string;
  GOOGLE_OAUTH_CLIENT_ID: string;
  GOOGLE_OAUTH_CLIENT_SECRET: string;
  GOOGLE_OAUTH_REDIRECT_URL: string;
  AUTH_APPLICATION_URL: string;
  AUTH_ALLOWED_EMAILS: string;
  AUTH_SUPERADMIN_EMAIL: string;
  GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY: string;
  R2_ENABLED?: string;
  R2_ENDPOINT?: string;
  R2_BUCKET?: string;
  R2_ACCESS_KEY_ID?: string;
  R2_SECRET_ACCESS_KEY?: string;
  OCR_ENABLED?: string;
  OCR_PROVIDER?: string;
  OCR_MODEL?: string;
  OCR_TIMEOUT?: string;
  OCR_MAX_REQUESTS_PER_HOUR?: string;
  OCR_MAX_PROVIDER_USAGE_PER_HOUR?: string;
  OCR_MAX_SOURCE_BYTES?: string;
};

/**
 * Proxies all HTTP traffic to the Go API container (API + River worker).
 * Worker secrets/vars come from the Durable Object env (not cloudflare:workers globals).
 */
export class ApiContainer extends Container<ApiEnv> {
  defaultPort = 8080;
  sleepAfter = "30m";
  enableInternet = true;

  constructor(ctx: DurableObjectState, env: ApiEnv) {
    super(ctx, env);
    this.envVars = {
      APP_ENV: "staging",
      HTTP_ADDRESS: ":8080",
      LOG_LEVEL: "info",
      AUTH_ENABLED: "true",
      AI_CHAT_ENABLED: "false",
      GOOGLE_FORMS_ENABLED: "false",
      DATABASE_URL: env.DATABASE_URL ?? "",
      GOOGLE_OAUTH_CLIENT_ID: env.GOOGLE_OAUTH_CLIENT_ID ?? "",
      GOOGLE_OAUTH_CLIENT_SECRET: env.GOOGLE_OAUTH_CLIENT_SECRET ?? "",
      GOOGLE_OAUTH_REDIRECT_URL: env.GOOGLE_OAUTH_REDIRECT_URL ?? "",
      AUTH_APPLICATION_URL: env.AUTH_APPLICATION_URL ?? "",
      AUTH_ALLOWED_EMAILS: env.AUTH_ALLOWED_EMAILS ?? "",
      AUTH_SUPERADMIN_EMAIL: env.AUTH_SUPERADMIN_EMAIL ?? "",
      GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY: env.GOOGLE_FORMS_TOKEN_ENCRYPTION_KEY ?? "",
      R2_ENABLED: env.R2_ENABLED ?? "false",
      R2_ENDPOINT: env.R2_ENDPOINT ?? "",
      R2_BUCKET: env.R2_BUCKET ?? "",
      R2_ACCESS_KEY_ID: env.R2_ACCESS_KEY_ID ?? "",
      R2_SECRET_ACCESS_KEY: env.R2_SECRET_ACCESS_KEY ?? "",
      OCR_ENABLED: env.OCR_ENABLED ?? "false",
      OCR_PROVIDER: env.OCR_PROVIDER ?? "",
      OCR_MODEL: env.OCR_MODEL ?? "",
      OCR_TIMEOUT: env.OCR_TIMEOUT ?? "90s",
      OCR_MAX_REQUESTS_PER_HOUR: env.OCR_MAX_REQUESTS_PER_HOUR ?? "10",
      OCR_MAX_PROVIDER_USAGE_PER_HOUR: env.OCR_MAX_PROVIDER_USAGE_PER_HOUR ?? "500000",
      OCR_MAX_SOURCE_BYTES: env.OCR_MAX_SOURCE_BYTES ?? "20971520",
    };
  }

  override onError(error: unknown) {
    console.error("ApiContainer error", error);
  }
}

export default {
  async fetch(request: Request, env: ApiEnv): Promise<Response> {
    const container = await getRandom(env.API_CONTAINER, INSTANCE_COUNT);
    return container.fetch(request);
  },
};
