import { mkdir, stat, rm } from "node:fs/promises";
import { join } from "node:path";
import type { PluginInput } from "@opencode-ai/plugin";
import type { GitConfig, PluginConfig } from "../types";

type BunShell = PluginInput["$"];

export interface WorkspaceInfo {
  name: string;
  path: string;
  created: boolean;
  git?: {
    url: string;
    branch: string;
    commit: string;
  };
}

export class WorkspaceManager {
  private config: PluginConfig;
  private shell: BunShell;

  constructor(config: PluginConfig, shell: BunShell) {
    this.config = config;
    this.shell = shell;
  }

  async resolve(name: string, gitConfig?: GitConfig): Promise<WorkspaceInfo> {
    const workspacePath = join(this.config.workspace.baseDir, name);
    const exists = await this.exists(workspacePath);

    if (exists) {
      const gitInfo = await this.getGitInfo(workspacePath);
      
      if (gitConfig?.pull !== false && gitInfo) {
        await this.gitPull(workspacePath);
        const updatedGitInfo = await this.getGitInfo(workspacePath);
        return {
          name,
          path: workspacePath,
          created: false,
          git: updatedGitInfo || undefined,
        };
      }

      return {
        name,
        path: workspacePath,
        created: false,
        git: gitInfo || undefined,
      };
    }

    if (gitConfig?.url) {
      await this.gitClone(workspacePath, gitConfig);
      const gitInfo = await this.getGitInfo(workspacePath);
      return {
        name,
        path: workspacePath,
        created: true,
        git: gitInfo || undefined,
      };
    }

    await mkdir(workspacePath, { recursive: true });

    return {
      name,
      path: workspacePath,
      created: true,
    };
  }

  private async exists(path: string): Promise<boolean> {
    try {
      await stat(path);
      return true;
    } catch {
      return false;
    }
  }

  private async gitClone(workspacePath: string, config: GitConfig): Promise<void> {
    const url = this.injectCredentials(config.url, config.token);
    const branch = config.branch || this.config.git.defaultBranch;

    try {
      if (config.ref) {
        await this.shell`git clone ${url} ${workspacePath}`;
        await this.shell`cd ${workspacePath} && git checkout ${config.ref}`;
      } else {
        await this.shell`git clone --branch ${branch} ${url} ${workspacePath}`;
      }
    } catch (err) {
      console.error("[station-plugin] git clone failed:", err);
      throw new Error(`git clone failed: ${err instanceof Error ? err.message : String(err)}`);
    }
  }

  private async gitPull(workspacePath: string): Promise<void> {
    try {
      await this.shell`cd ${workspacePath} && git pull --ff-only`;
    } catch (err) {
      console.warn("[station-plugin] git pull failed in", workspacePath, err);
    }
  }

  private async getGitInfo(workspacePath: string): Promise<{ url: string; branch: string; commit: string } | null> {
    try {
      const urlResult = await this.shell`cd ${workspacePath} && git remote get-url origin`;
      const branchResult = await this.shell`cd ${workspacePath} && git branch --show-current`;
      const commitResult = await this.shell`cd ${workspacePath} && git rev-parse HEAD`;

      return {
        url: urlResult.text().trim(),
        branch: branchResult.text().trim(),
        commit: commitResult.text().trim(),
      };
    } catch {
      return null;
    }
  }

  async isGitDirty(workspacePath: string): Promise<boolean> {
    try {
      const result = await this.shell`cd ${workspacePath} && git status --porcelain`;
      return result.text().trim().length > 0;
    } catch {
      return false;
    }
  }

  private injectCredentials(url: string, token?: string): string {
    if (!token) return url;

    if (url.startsWith("https://github.com/")) {
      return url.replace("https://github.com/", `https://x-access-token:${token}@github.com/`);
    }
    if (url.startsWith("https://gitlab.com/")) {
      return url.replace("https://gitlab.com/", `https://oauth2:${token}@gitlab.com/`);
    }
    return url;
  }

  async cleanup(name: string): Promise<void> {
    const workspacePath = join(this.config.workspace.baseDir, name);
    try {
      await rm(workspacePath, { recursive: true, force: true });
    } catch (err) {
      console.error(`[station-plugin] Failed to cleanup workspace ${name}:`, err);
    }
  }
}
