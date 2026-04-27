const assert = require("node:assert/strict");
const test = require("node:test");

const { buildPublishCommand, getPrereleaseTag } = require("./npm-publish");

test("getPrereleaseTag returns empty for stable versions", () => {
  assert.equal(getPrereleaseTag("1.2.3"), "");
  assert.equal(getPrereleaseTag("v1.2.3"), "");
});

test("getPrereleaseTag uses named prerelease channel", () => {
  assert.equal(getPrereleaseTag("1.2.3-alpha.2"), "alpha");
  assert.equal(getPrereleaseTag("v1.2.3-rc.1"), "rc");
  assert.equal(getPrereleaseTag("1.2.3-preview-build.4"), "preview-build");
});

test("getPrereleaseTag falls back to next for numeric prerelease identifiers", () => {
  assert.equal(getPrereleaseTag("1.2.3-0"), "next");
  assert.equal(getPrereleaseTag("1.2.3-0.20260427"), "next");
});

test("buildPublishCommand includes prerelease tag and dry run flags", () => {
  assert.equal(
    buildPublishCommand({ accessPublic: true, dryRun: true, tag: "alpha" }),
    "npm publish --access public --tag alpha --dry-run",
  );
  assert.equal(buildPublishCommand({ dryRun: false, tag: "" }), "npm publish");
});
