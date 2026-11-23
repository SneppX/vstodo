#!/usr/bin/env node
"use strict";

const fs = require("fs");
const path = require("path");
const https = require("https");
const { spawnSync } = require("child_process");

const pkg = require("./package.json");
const version = process.env.VSTODO_VERSION || pkg.version;
const repo = process.env.VSTODO_REPO || "SneppX/vstodo";

function platformAsset() {
  const platform = process.platform;
  const arch = process.arch;
  const goos = { darwin: "darwin", linux: "linux", win32: "windows" }[platform];
  const goarch = { x64: "amd64", arm64: "arm64" }[arch];
  if (!goos || !goarch) {
    throw new Error(`Unsupported platform: ${platform}/${arch}`);
  }
  const ext = platform === "win32" ? "zip" : "tar.gz";
  const name = `vstodo_${version}_${goos}_${goarch}.${ext}`;
  const url = `https://github.com/${repo}/releases/download/v${version}/${name}`;
  return { name, url, ext };
}

function download(url, dest) {
  return new Promise((resolve, reject) => {
    const file = fs.createFileStream(dest);
    https
      .get(url, (res) => {
        if (res.statusCode >= 300 && res.statusCode < 400 && res.headers.location) {
          file.close();
          fs.unlinkSync(dest);
          return resolve(download(res.headers.location, dest));
        }
        if (res.statusCode !== 200) {
          file.close();
          fs.unlinkSync(dest);
          return reject(new Error(`Download failed ${res.statusCode}: ${url}`));
        }
        res.pipe(file);
        file.on("finish", () => file.close(resolve));
      })
      .on("error", (err) => {
        file.close();
        try { fs.unlinkSync(dest); } catch {}
        reject(err);
      });
  });
}

function extract(archive, ext, outDir) {
  fs.mkdirSync(outDir, { recursive: true });
  if (ext === "zip") {
    if (process.platform === "win32") {
      const r = spawnSync("powershell", ["-NoProfile", "-Command", `Expand-Archive -Force -Path '${archive}' -DestinationPath '${outDir}'`], { stdio: "inherit" });
      if (r.status !== 0) throw new Error("Failed to extract zip");
      return;
    }
    const r = spawnSync("unzip", ["-o", archive, "-d", outDir], { stdio: "inherit" });
    if (r.status !== 0) throw new Error("Failed to extract zip (unzip required)");
    return;
  }
  const r = spawnSync("tar", ["-xzf", archive, "-C", outDir], { stdio: "inherit" });
  if (r.status !== 0) throw new Error("Failed to extract tar.gz");
}

async function main() {
  if (version === "0.0.0") {
    console.log("vstodo npm wrapper: development version (0.0.0), skipping binary download.");
    console.log("Install a released version or set VSTODO_VERSION to download a binary.");
    return;
  }
  const { name, url, ext } = platformAsset();
  const vendorDir = path.join(__dirname, "vendor");
  const archive = path.join(__dirname, name);
  console.log(`Downloading ${url}`);
  await download(url, archive);
  extract(archive, ext, vendorDir);
  fs.unlinkSync(archive);
  const binName = process.platform === "win32" ? "vstodo.exe" : "vstodo";
  const binPath = path.join(vendorDir, binName);
  if (!fs.existsSync(binPath)) {
    throw new Error(`Expected binary not found after extract: ${binPath}`);
  }
  if (process.platform !== "win32") fs.chmodSync(binPath, 0o755);
  console.log(`vstodo installed: ${binPath}`);
}

main().catch((err) => {
  console.error(err.message);
  process.exit(1);
});
