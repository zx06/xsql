#!/usr/bin/env node

const { execFileSync, execSync } = require("child_process");
const fs = require("fs");
const os = require("os");
const path = require("path");

const { PLATFORM_MAP } = require("./release-artifacts");

function readJSON(filePath) {
  return JSON.parse(fs.readFileSync(filePath, "utf8"));
}

function findBinaryArtifact(distDir, goos, goarch) {
  const artifactsPath = path.join(distDir, "artifacts.json");
  if (!fs.existsSync(artifactsPath)) {
    throw new Error(`missing ${artifactsPath}; run GoReleaser build first`);
  }
  const artifacts = readJSON(artifactsPath);
  const match = artifacts.find(
    (item) =>
      item.type === "Binary" &&
      item.goos === goos &&
      item.goarch === goarch &&
      item.path,
  );
  if (!match) {
    throw new Error(`no binary artifact for ${goos}/${goarch} in ${artifactsPath}`);
  }
  const relative = match.path.replace(/^dist[\\/]/, "");
  const binaryPath = path.isAbsolute(match.path)
    ? match.path
    : path.join(distDir, relative);
  if (!fs.existsSync(binaryPath)) {
    throw new Error(`binary not found at ${binaryPath}`);
  }
  return binaryPath;
}

function ensureExecutable(binaryPath) {
  if (process.platform === "win32") {
    return;
  }
  fs.chmodSync(binaryPath, 0o755);
}

function runBinarySmoke(binaryPath) {
  ensureExecutable(binaryPath);
  const commands = [
    ["version"],
    ["spec", "--format", "json"],
    ["profile", "list", "--format", "json"],
  ];
  for (const args of commands) {
    console.log(`> ${binaryPath} ${args.join(" ")}`);
    execFileSync(binaryPath, args, { stdio: "inherit", env: process.env });
  }
}

function copyBinaryToNpm(distDir, version) {
  const repoRoot = path.join(__dirname, "..");
  const npmDir = path.join(repoRoot, "npm");
  for (const platform of PLATFORM_MAP) {
    const src = findBinaryArtifact(distDir, platform.goos, platform.goarch);
    const binDir = path.join(npmDir, platform.npm, "bin");
    const dest = path.join(binDir, `xsql${platform.ext}`);
    fs.mkdirSync(binDir, { recursive: true });
    fs.copyFileSync(src, dest);
    if (platform.ext === "") {
      fs.chmodSync(dest, 0o755);
    }
  }
  updateNpmVersions(npmDir, version);
}

function updateNpmVersions(npmDir, version) {
  const mainPkg = path.join(npmDir, "xsql", "package.json");
  const main = readJSON(mainPkg);
  main.version = version;
  if (main.optionalDependencies) {
    for (const dep of Object.keys(main.optionalDependencies)) {
      main.optionalDependencies[dep] = version;
    }
  }
  fs.writeFileSync(mainPkg, JSON.stringify(main, null, 2) + "\n");
  for (const platform of PLATFORM_MAP) {
    const pkgPath = path.join(npmDir, platform.npm, "package.json");
    const pkg = readJSON(pkgPath);
    pkg.version = version;
    fs.writeFileSync(pkgPath, JSON.stringify(pkg, null, 2) + "\n");
  }
}

function resolveVersion(distDir) {
  const metadataPath = path.join(distDir, "metadata.json");
  if (fs.existsSync(metadataPath)) {
    return readJSON(metadataPath).version;
  }
  return "0.0.0-smoke";
}

function runNpm(args, cwd) {
  if (process.platform === "win32") {
    execSync(`npm ${args.map((a) => `"${a}"`).join(" ")}`, {
      cwd,
      stdio: "inherit",
      shell: true,
    });
    return;
  }
  execFileSync("npm", args, { cwd, stdio: "inherit" });
}

function runNpx(args, cwd) {
  if (process.platform === "win32") {
    execSync(`npx ${args.map((a) => `"${a}"`).join(" ")}`, {
      cwd,
      stdio: "inherit",
      shell: true,
    });
    return;
  }
  execFileSync("npx", args, { cwd, stdio: "inherit" });
}

function packTarball(cwd) {
  const output =
    process.platform === "win32"
      ? execSync("npm pack --json", { cwd, encoding: "utf8", shell: true })
      : execFileSync("npm", ["pack", "--json"], {
          cwd,
          encoding: "utf8",
        });
  const parsed = JSON.parse(output);
  const entry = Array.isArray(parsed) ? parsed[0] : parsed;
  return path.resolve(cwd, entry.filename);
}

function smokeNpmFromDist(distDir) {
  const repoRoot = path.join(__dirname, "..");
  const npmDir = path.join(repoRoot, "npm");
  const version = resolveVersion(distDir);
  copyBinaryToNpm(distDir, version);

  const key = `${os.platform()}-${os.arch()}`;
  const platform = PLATFORM_MAP.find((p) => p.npm === key);
  if (!platform) {
    throw new Error(`unsupported npm smoke platform: ${key}`);
  }

  const platformDir = path.join(npmDir, platform.npm);
  const mainDir = path.join(npmDir, "xsql");
  const platformTgz = packTarball(platformDir);
  const mainTgz = packTarball(mainDir);

  const workDir = fs.mkdtempSync(path.join(os.tmpdir(), "xsql-npm-smoke-"));
  console.log(`npm smoke workdir: ${workDir}`);
  runNpm(["init", "-y"], workDir);
  runNpm(["install", platformTgz, mainTgz], workDir);
  runNpx(["xsql", "version"], workDir);
  runNpx(["xsql", "spec", "--format", "json"], workDir);
}

function main() {
  const [mode, ...rest] = process.argv.slice(2);
  const distDir = path.resolve(rest[0] || "dist");

  if (mode === "binary") {
    const [goos, goarch] = rest.slice(1);
    if (!goos || !goarch) {
      console.error("Usage: node scripts/release-smoke.js binary <distDir> <goos> <goarch>");
      process.exit(1);
    }
    runBinarySmoke(findBinaryArtifact(distDir, goos, goarch));
    return;
  }

  if (mode === "npm") {
    smokeNpmFromDist(distDir);
    return;
  }

  console.error("Usage:");
  console.error("  node scripts/release-smoke.js binary [distDir] <goos> <goarch>");
  console.error("  node scripts/release-smoke.js npm [distDir]");
  process.exit(1);
}

if (require.main === module) {
  try {
    main();
  } catch (err) {
    console.error(err.message || err);
    process.exit(1);
  }
}

module.exports = {
  findBinaryArtifact,
  resolveVersion,
};
