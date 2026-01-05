/**
 * Station Skill Plugin for OpenCode
 * 
 * Provides skills that teach OpenCode how to use Station CLI for AI agent orchestration.
 * 
 * Skills included:
 * - station: Core CLI commands, agents, workflows, MCP, deployment
 * - station-config: Configuration management via CLI and browser UI
 */

import { readFileSync } from "fs";
import { join, dirname } from "path";
import { fileURLToPath } from "url";

const __filename = fileURLToPath(import.meta.url);
const __dirname = dirname(__filename);

const skillsDir = join(__dirname, "..", "skills");

let stationSkill: string;
let stationConfigSkill: string;

try {
  stationSkill = readFileSync(join(skillsDir, "station.md"), "utf-8");
  stationConfigSkill = readFileSync(join(skillsDir, "station-config.md"), "utf-8");
} catch (err) {
  console.error("[station-skill] Failed to load skill files:", err);
  stationSkill = "# Station\n\nSkill file not found.";
  stationConfigSkill = "# Station Config\n\nSkill file not found.";
}

export interface PluginInput {
  client: unknown;
  $: unknown;
  directory: string;
}

export interface Hooks {
  skill?: Record<string, { content: string; description?: string }>;
}

const plugin = async (_input: PluginInput): Promise<Hooks> => {
  console.log("[station-skill] Loaded");
  
  return {
    skill: {
      station: {
        content: stationSkill,
        description: "Station CLI for AI agent orchestration - agents, workflows, MCP tools, deployment",
      },
      "station-config": {
        content: stationConfigSkill,
        description: "Station configuration management via CLI and browser UI",
      },
    },
  };
};

export default plugin;
