#!/usr/bin/env node
/**
 * Staging SPA deploys must bake VITE_API_BASE_URL into the bundle.
 * Without it, Google login navigates to same-origin /auth/login (SPA) and appears broken.
 */
import { existsSync, readFileSync } from "node:fs";
import { resolve } from "node:path";

const root = resolve(import.meta.dirname, "..");
const envPath = resolve(root, ".env");

function loadDotEnv(path) {
  if (!existsSync(path)) return;
  for (const line of readFileSync(path, "utf8").split(/\r?\n/)) {
    const trimmed = line.trim();
    if (!trimmed || trimmed.startsWith("#")) continue;
    const eq = trimmed.indexOf("=");
    if (eq <= 0) continue;
    const key = trimmed.slice(0, eq).trim();
    let value = trimmed.slice(eq + 1).trim();
    if (
      (value.startsWith('"') && value.endsWith('"')) ||
      (value.startsWith("'") && value.endsWith("'"))
    ) {
      value = value.slice(1, -1);
    }
    if (process.env[key] === undefined) process.env[key] = value;
  }
}

loadDotEnv(envPath);

const base = (process.env.VITE_API_BASE_URL || "").trim().replace(/\/$/, "");
if (!/^https:\/\//i.test(base)) {
  console.error(
    "deploy:web:staging requires VITE_API_BASE_URL to be an https origin (set in .env or the environment).",
  );
  process.exit(1);
}

process.stdout.write(`${base}\n`);
