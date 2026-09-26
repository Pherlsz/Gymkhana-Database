#!/usr/bin/env node
/**
 * Production SPA bundles must reach the API Worker.
 * Workers Builds runs `pnpm build:web` without VITE_API_BASE_URL, so the
 * workers.dev hostname inference in apiURL must remain in the dist, or an
 * absolute https API origin must be baked in.
 */
import { readdirSync, readFileSync } from "node:fs";
import { join, resolve } from "node:path";

const distAssets = resolve(import.meta.dirname, "../apps/web/dist/assets");
const files = readdirSync(distAssets).filter((name) => name.endsWith(".js"));
if (files.length === 0) {
  console.error("assert-spa-api-origin: no JS assets in apps/web/dist/assets");
  process.exit(1);
}

const bundle = files.map((name) => readFileSync(join(distAssets, name), "utf8")).join("\n");
const hasBakedHttpsApi = /https:\/\/staging-gymkhana-api\.[^"'`\s]+/.test(bundle);
const hasWorkersDevInference =
  bundle.includes("staging-gymkhana-database.") && bundle.includes("staging-gymkhana-api.");

if (!hasBakedHttpsApi && !hasWorkersDevInference) {
  console.error(
    "assert-spa-api-origin: production bundle has neither a baked https API origin nor workers.dev SPA→API inference. Google login would navigate to same-origin /auth/login.",
  );
  process.exit(1);
}

process.stdout.write(
  hasBakedHttpsApi
    ? "assert-spa-api-origin: baked API origin present\n"
    : "assert-spa-api-origin: workers.dev inference present\n",
);
