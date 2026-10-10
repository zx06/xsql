const assert = require("node:assert/strict");
const fs = require("node:fs");
const os = require("node:os");
const path = require("node:path");
const test = require("node:test");

const { findBinaryArtifact, resolveVersion } = require("./release-smoke");

test("findBinaryArtifact resolves binary path from artifacts.json", () => {
  const distDir = fs.mkdtempSync(path.join(os.tmpdir(), "xsql-dist-"));
  const binaryRel = path.join("dist", "xsql_linux_amd64_v1", "xsql");
  const binaryAbs = path.join(distDir, "xsql_linux_amd64_v1", "xsql");
  fs.mkdirSync(path.dirname(binaryAbs), { recursive: true });
  fs.writeFileSync(binaryAbs, "");
  fs.writeFileSync(
    path.join(distDir, "artifacts.json"),
    JSON.stringify([
      {
        type: "Binary",
        goos: "linux",
        goarch: "amd64",
        path: binaryRel,
      },
    ]),
  );

  assert.equal(findBinaryArtifact(distDir, "linux", "amd64"), binaryAbs);
  assert.equal(resolveVersion(distDir), "0.0.0-smoke");
});

test("findBinaryArtifact rejects unknown platform", () => {
  const distDir = fs.mkdtempSync(path.join(os.tmpdir(), "xsql-dist-"));
  fs.writeFileSync(path.join(distDir, "artifacts.json"), "[]");
  assert.throws(
    () => findBinaryArtifact(distDir, "plan9", "amd64"),
    /unsupported smoke platform/,
  );
});
