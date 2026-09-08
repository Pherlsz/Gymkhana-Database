#!/usr/bin/env node

/**
 * scripts/check-bundle.mjs
 *
 * Automated bundle & import audit script.
 * Enforces:
 * 1. Route-level CSS decoupling (no feature CSS leaked into main.tsx)
 * 2. No revived barrel files or dead proxies
 * 3. TypeScript compilation with 0 errors
 * 4. Production build performance budgets (raw & gzip sizes)
 */

import { execSync } from "node:child_process";
import fs from "node:fs";
import path from "node:path";
import zlib from "node:zlib";

const ROOT_DIR = process.cwd();
const WEB_DIR = path.join(ROOT_DIR, "apps/web");
const DIST_ASSETS = path.join(WEB_DIR, "dist/assets");

// Performance Budgets (in KB)
const BUDGETS = {
  entryCssMaxKb: 65, // Max initial CSS (currently ~56 KB)
  entryJsMaxKb: 110, // Max initial JS (currently ~99.8 KB)
  routeJsMaxKb: 75, // Max individual route/feature chunk
};

// Forbidden CSS in root entry (must be code-split into their respective routes)
const FORBIDDEN_ROOT_CSS = [
  "home.css",
  "tables.css",
  "cadastro.css",
  "search.css",
  "google-forms.css",
  "attachments.css",
  "operations.css",
];

// Dead barrel files that must never be re-introduced
const FORBIDDEN_BARREL_IMPORTS = [
  "lib/cadastro/modes/index",
  "lib/cadastro/components/index",
  "lib/search/index",
  "i18n/v1/catalog",
];

function log(msg, color = "\x1b[0m") {
  console.log(`${color}${msg}\x1b[0m`);
}

function fail(msg) {
  console.error(`\x1b[31m✖ ERROR:\x1b[0m ${msg}`);
  process.exit(1);
}

function checkRootCssImports() {
  log("\n🔍 [1/4] Verificando desacoplamento de CSS por rota...", "\x1b[36m");
  const mainTsxPath = path.join(WEB_DIR, "src/main.tsx");
  const content = fs.readFileSync(mainTsxPath, "utf-8");

  for (const forbidden of FORBIDDEN_ROOT_CSS) {
    if (content.includes(forbidden)) {
      fail(
        `Regressão de bundle detectada em main.tsx: "${forbidden}" não deve ser importado globalmente no root. Importe diretamente no componente da rota correspondente.`,
      );
    }
  }
  log("  ✔ CSS de rotas devidamente desacoplado (sem vazamento para o root).", "\x1b[32m");
}

function checkBarrelImports() {
  log("\n🔍 [2/4] Auditando anti-padrões e barrel files proxy...", "\x1b[36m");
  const srcDir = path.join(WEB_DIR, "src");

  function scanDir(dir) {
    const entries = fs.readdirSync(dir, { withFileTypes: true });
    for (const entry of entries) {
      const fullPath = path.join(dir, entry.name);
      if (entry.isDirectory()) {
        scanDir(fullPath);
      } else if (/\.(ts|tsx)$/.test(entry.name)) {
        const code = fs.readFileSync(fullPath, "utf-8");
        for (const barrel of FORBIDDEN_BARREL_IMPORTS) {
          if (code.includes(barrel)) {
            fail(
              `Import de barrel file obsoleto detectado em ${path.relative(WEB_DIR, fullPath)}: "${barrel}". Importe diretamente do módulo específico.`,
            );
          }
        }
      }
    }
  }

  scanDir(srcDir);
  log("  ✔ Nenhum proxy/barrel file proibido detectado.", "\x1b[32m");
}

function runTypecheck() {
  log("\n🔍 [3/4] Validando tipagem TypeScript...", "\x1b[36m");
  try {
    execSync("npx tsc -p tsconfig.json --noEmit", {
      cwd: WEB_DIR,
      stdio: "inherit",
    });
    log("  ✔ 0 erros de TypeScript.", "\x1b[32m");
  } catch {
    fail("Falha na validação de tipos TypeScript.");
  }
}

function runBuildAndCheckBudgets() {
  log("\n🔍 [4/4] Compilando produção e auditando orçamentos de bundle...", "\x1b[36m");
  try {
    execSync("npx vite build", {
      cwd: WEB_DIR,
      stdio: "pipe",
    });
  } catch (err) {
    fail(`Falha no build de produção do Vite: ${err.message}`);
  }

  if (!fs.existsSync(DIST_ASSETS)) {
    fail("Diretório dist/assets não encontrado após o build.");
  }

  const files = fs.readdirSync(DIST_ASSETS);
  const rows = [];
  let entryCssSizeKb = 0;
  let entryJsSizeKb = 0;

  for (const file of files) {
    const filePath = path.join(DIST_ASSETS, file);
    const rawBytes = fs.statSync(filePath).size;
    const gzipBytes = zlib.gzipSync(fs.readFileSync(filePath)).length;

    const rawKb = (rawBytes / 1024).toFixed(2);
    const gzipKb = (gzipBytes / 1024).toFixed(2);

    rows.push({ file, rawKb: parseFloat(rawKb), gzipKb: parseFloat(gzipKb) });

    if (file.startsWith("index-") && file.endsWith(".css")) {
      entryCssSizeKb = parseFloat(rawKb);
    }
    if (file.startsWith("index-") && file.endsWith(".js")) {
      entryJsSizeKb = parseFloat(rawKb);
    }
  }

  // Ordenar por tamanho decrescente
  rows.sort((a, b) => b.rawKb - a.rawKb);

  log("\n📦 Resumo dos Chunks de Produção:");
  console.table(
    rows.map((r) => ({
      Arquivo: r.file,
      "Tamanho (KB)": `${r.rawKb.toFixed(2)} KB`,
      "Gzip (KB)": `${r.gzipKb.toFixed(2)} KB`,
    })),
  );

  // Verificação de Budgets
  log("\n🎯 Verificação de Orçamentos (Budgets):");

  if (entryCssSizeKb > BUDGETS.entryCssMaxKb) {
    fail(
      `CSS inicial excede o limite! Tamanho: ${entryCssSizeKb} KB > Máximo permitido: ${BUDGETS.entryCssMaxKb} KB.`,
    );
  } else {
    log(`  ✔ CSS Inicial: ${entryCssSizeKb} KB <= ${BUDGETS.entryCssMaxKb} KB`, "\x1b[32m");
  }

  if (entryJsSizeKb > BUDGETS.entryJsMaxKb) {
    fail(
      `JS inicial excede o limite! Tamanho: ${entryJsSizeKb} KB > Máximo permitido: ${BUDGETS.entryJsMaxKb} KB.`,
    );
  } else {
    log(`  ✔ JS Inicial: ${entryJsSizeKb} KB <= ${BUDGETS.entryJsMaxKb} KB`, "\x1b[32m");
  }

  // Verificar chunks de rotas da aplicação (excluindo vendors isolados e chunk de i18n)
  for (const r of rows) {
    if (r.file.startsWith("vendor-") || r.file.startsWith("i18n-") || r.file.startsWith("index-"))
      continue;
    if (r.file.endsWith(".js") && r.rawKb > BUDGETS.routeJsMaxKb) {
      fail(
        `Chunk de rota "${r.file}" excede o limite de ${BUDGETS.routeJsMaxKb} KB (${r.rawKb} KB). Considere code-splitting.`,
      );
    }
  }

  log(`  ✔ Chunks de rota dentro do orçamento (<= ${BUDGETS.routeJsMaxKb} KB).`, "\x1b[32m");
}

function main() {
  log("🚀 Iniciando Auditoria de Performance & Imports...", "\x1b[1m\x1b[35m");
  checkRootCssImports();
  checkBarrelImports();
  runTypecheck();
  runBuildAndCheckBudgets();
  log(
    "\n✨ Todos os checks de imports e performance de bundle passaram com sucesso!\n",
    "\x1b[1m\x1b[32m",
  );
}

main();
