#!/usr/bin/env node

const { execSync } = require("child_process");
const fs = require("fs");
const path = require("path");
const https = require("https");
const http = require("http");
const { createWriteStream, mkdirSync } = require("fs");

const REPO = "zx06/xsql";
const NPM_DIR = path.join(__dirname, "..", "npm");

const PLATFORM_MAP = [
  { goos: "linux",   goarch: "amd64", npm: "linux-x64",   ext: "", archive: "tar.gz" },
  { goos: "linux",   goarch: "arm64", npm: "linux-arm64",  ext: "", archive: "tar.gz" },
  { goos: "darwin",  goarch: "amd64", npm: "darwin-x64",   ext: "", archive: "tar.gz" },
  { goos: "darwin",  goarch: "arm64", npm: "darwin-arm64",  ext: "", archive: "tar.gz" },
  { goos: "windows", goarch: "amd64", npm: "win32-x64",    ext: ".exe", archive: "zip" },
  { goos: "windows", goarch: "arm64", npm: "win32-arm64",   ext: ".exe", archive: "zip" },
];

function run(cmd, opts = {}) {
  console.log(`$ ${cmd}`);
  return execSync(cmd, { stdio: "inherit", ...opts });
}

function updateVersion(pkgPath, version) {
  const pkg = JSON.parse(fs.readFileSync(pkgPath, "utf8"));
  pkg.version = version;
  if (pkg.optionalDependencies) {
    for (const dep of Object.keys(pkg.optionalDependencies)) {
      pkg.optionalDependencies[dep] = version;
    }
  }
  fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
  console.log(`Updated ${pkgPath} to v${version}`);
}

function downloadFile(url, dest) {
  return new Promise((resolve, reject) => {
    const follow = (url) => {
      const client = url.startsWith("https") ? https : http;
      client.get(url, { headers: { "User-Agent": "xsql-npm-publish" } }, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          follow(res.headers.location);
          return;
        }
        if (res.statusCode !== 200) {
          reject(new Error(`Download failed: ${res.statusCode} for ${url}`));
          return;
        }
        const file = createWriteStream(dest);
        res.pipe(file);
        file.on("finish", () => { file.close(); resolve(); });
        file.on("error", reject);
      }).on("error", reject);
    };
    follow(url);
  });
}

async function extractBinary(archivePath, archiveType, binaryName, destDir) {
  mkdirSync(destDir, { recursive: true });
  if (archiveType === "tar.gz") {
    run(`tar -xzf "${archivePath}" -C "${destDir}" "${binaryName}"`);
  } else {
    run(`unzip -o "${archivePath}" "${binaryName}" -d "${destDir}"`);
  }
}

async function main() {
  const args = process.argv.slice(2);
  const dryRun = args.includes("--dry-run");
  const version = args.find((a) => !a.startsWith("--"));

  if (!version) {
    console.error("Usage: node scripts/npm-publish.js <version> [--dry-run]");
    console.error("Example: node scripts/npm-publish.js 1.2.3");
    process.exit(1);
  }

  const cleanVersion = version.replace(/^v/, "");
  const tmpDir = path.join(__dirname, "..", ".npm-tmp");
  mkdirSync(tmpDir, { recursive: true });

  console.log(`\nPublishing xsql v${cleanVersion} to npm${dryRun ? " (dry-run)" : ""}...\n`);

  console.log("==> Updating versions...");
  updateVersion(path.join(NPM_DIR, "xsql", "package.json"), cleanVersion);
  for (const p of PLATFORM_MAP) {
    updateVersion(path.join(NPM_DIR, p.npm, "package.json"), cleanVersion);
  }

  console.log("\n==> Downloading binaries from GitHub release...");
  for (const p of PLATFORM_MAP) {
    const archiveName = `xsql_${cleanVersion}_${p.goos}_${p.goarch}.${p.archive}`;
    const url = `https://github.com/${REPO}/releases/download/v${cleanVersion}/${archiveName}`;
    const archivePath = path.join(tmpDir, archiveName);
    const binDir = path.join(NPM_DIR, p.npm, "bin");

    console.log(`  Downloading ${archiveName}...`);
    await downloadFile(url, archivePath);

    console.log(`  Extracting to ${p.npm}/bin/...`);
    const binaryName = `xsql${p.ext}`;
    await extractBinary(archivePath, p.archive, binaryName, binDir);

    if (p.ext === "") {
      fs.chmodSync(path.join(binDir, binaryName), 0o755);
    }
  }

  console.log("\n==> Publishing platform packages...");
  const publishFlag = dryRun ? "--dry-run" : "";
  for (const p of PLATFORM_MAP) {
    const pkgDir = path.join(NPM_DIR, p.npm);
    console.log(`  Publishing @xsql-cli/${p.npm}...`);
    run(`npm publish --access public ${publishFlag}`, { cwd: pkgDir });
  }

  console.log("\n==> Publishing main package...");
  run(`npm publish ${publishFlag}`, { cwd: path.join(NPM_DIR, "xsql") });

  fs.rmSync(tmpDir, { recursive: true, force: true });

  console.log(`\nDone! xsql-cli v${cleanVersion} published to npm.`);
}

main().catch((err) => {
  console.error(err);
  process.exit(1);
});
