#!/usr/bin/env node
"use strict";

const path = require("path");
const fs = require("fs");
const { spawn } = require("child_process");

const binName = process.platform === "win32" ? "vstodo.exe" : "vstodo";
const vendorBin = path.join(__dirname, "..", "vendor", binName);

if (!fs.existsSync(vendorBin)) {
  console.error("vstodo binary not found. Try reinstalling, or install a released version.");
  console.error(`Expected: ${vendorBin}`);
  process.exit(1);
}

const child = spawn(vendorBin, process.argv.slice(2), { stdio: "inherit" });
child.on("exit", (code, signal) => {
  if (signal) process.kill(process.pid, signal);
  process.exit(code == null ? 1 : code);
});
child.on("error", (err) => {
  console.error(err.message);
  process.exit(1);
});
