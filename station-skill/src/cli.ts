#!/usr/bin/env bun

import { existsSync, readFileSync, writeFileSync, mkdirSync } from "fs";
import { join } from "path";
import { homedir } from "os";

const PACKAGE_NAME = "@cloudshipai/station-skill";
const CONFIG_DIR = join(homedir(), ".config", "opencode");
const CONFIG_FILE = join(CONFIG_DIR, "opencode.json");

interface OpenCodeConfig {
  plugin?: string[];
  [key: string]: unknown;
}

function loadConfig(): OpenCodeConfig {
  if (!existsSync(CONFIG_FILE)) {
    return {};
  }
  try {
    return JSON.parse(readFileSync(CONFIG_FILE, "utf-8"));
  } catch {
    return {};
  }
}

function saveConfig(config: OpenCodeConfig): void {
  mkdirSync(CONFIG_DIR, { recursive: true });
  writeFileSync(CONFIG_FILE, JSON.stringify(config, null, 2));
}

function install(): void {
  const config = loadConfig();
  const plugins = config.plugin || [];
  
  if (plugins.includes(PACKAGE_NAME)) {
    console.log(`${PACKAGE_NAME} is already installed`);
    return;
  }
  
  config.plugin = [...plugins, PACKAGE_NAME];
  saveConfig(config);
  console.log(`Installed ${PACKAGE_NAME}`);
  console.log(`Restart OpenCode to load the station skills`);
}

function uninstall(): void {
  const config = loadConfig();
  const plugins = config.plugin || [];
  
  config.plugin = plugins.filter((p) => p !== PACKAGE_NAME && !p.includes("station-skill"));
  saveConfig(config);
  console.log(`Uninstalled ${PACKAGE_NAME}`);
}

const command = process.argv[2];

switch (command) {
  case "install":
    install();
    break;
  case "uninstall":
    uninstall();
    break;
  default:
    console.log(`Usage: bunx ${PACKAGE_NAME} <install|uninstall>`);
    process.exit(1);
}
