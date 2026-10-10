const PLATFORM_MAP = [
  { goos: "linux", goarch: "amd64", npm: "linux-x64", ext: "", archive: "tar.gz" },
  { goos: "linux", goarch: "arm64", npm: "linux-arm64", ext: "", archive: "tar.gz" },
  { goos: "darwin", goarch: "amd64", npm: "darwin-x64", ext: "", archive: "tar.gz" },
  { goos: "darwin", goarch: "arm64", npm: "darwin-arm64", ext: "", archive: "tar.gz" },
  { goos: "windows", goarch: "amd64", npm: "win32-x64", ext: ".exe", archive: "zip" },
  { goos: "windows", goarch: "arm64", npm: "win32-arm64", ext: ".exe", archive: "zip" },
];

module.exports = { PLATFORM_MAP };
