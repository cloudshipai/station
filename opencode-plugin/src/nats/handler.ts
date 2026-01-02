import type { createOpencodeClient, Part, TextPart, ToolPart } from "@opencode-ai/sdk";
import type { PluginInput } from "@opencode-ai/plugin";
import type { NATSClient } from "./client";
import { EventPublisher } from "./publisher";
import { WorkspaceManager } from "../workspace/manager";
import { SessionManager } from "../session/manager";
import type { CodingTask, PluginConfig } from "../types";

type BunShell = PluginInput["$"];
type OpencodeClient = ReturnType<typeof createOpencodeClient>;

export class TaskHandler {
  private client: OpencodeClient;
  private nats: NATSClient | null;
  private workspaceManager: WorkspaceManager;
  private sessionManager: SessionManager;
  private config: PluginConfig;

  constructor(
    client: OpencodeClient,
    nats: NATSClient | null,
    shell: BunShell,
    config: PluginConfig
  ) {
    this.client = client;
    this.nats = nats;
    this.config = config;
    this.workspaceManager = new WorkspaceManager(config, shell);
    this.sessionManager = new SessionManager(client, nats);
  }

  async handle(taskData: string): Promise<void> {
    let task: CodingTask;
    try {
      task = JSON.parse(taskData) as CodingTask;
    } catch (err) {
      console.error("[station-plugin] Failed to parse task:", err);
      return;
    }

    const publisher = new EventPublisher(
      this.nats,
      task.taskID,
      task.callback.streamSubject,
      task.callback.resultSubject
    );

    const startTime = Date.now();

    try {
      const workspace = await this.workspaceManager.resolve(
        task.workspace.name,
        task.workspace.git
      );

      await publisher.stream(workspace.created ? "workspace_created" : "workspace_reused", {
        content: workspace.path,
      });

      if (workspace.git) {
        await publisher.stream(workspace.created ? "git_clone" : "git_pull", {
          git: workspace.git,
        });
      }

      const shouldContinue = task.session.continue !== false;
      const session = await this.sessionManager.resolve(
        task.session.name,
        workspace.path,
        shouldContinue
      );

      await publisher.stream(session.created ? "session_created" : "session_reused", {
        session: {
          name: session.name,
          opencodeID: session.opencodeID,
        },
      });

      if (workspace.git) {
        await this.sessionManager.updateGitInfo(task.session.name, {
          url: workspace.git.url,
          branch: workspace.git.branch,
          lastCommit: workspace.git.commit,
        });
      }

      await publisher.stream("prompt_sent", { content: task.prompt });

      const response = await this.executePrompt(
        session.opencodeID,
        workspace.path,
        task,
        publisher
      );

      await this.sessionManager.incrementMessageCount(task.session.name);
      const updatedSession = await this.sessionManager.getSessionInfo(task.session.name);
      const isDirty = await this.workspaceManager.isGitDirty(workspace.path);

      await publisher.result({
        status: "completed",
        result: response.text,
        session: {
          name: task.session.name,
          opencodeID: session.opencodeID,
          messageCount: updatedSession?.messageCount || session.messageCount + 1,
        },
        workspace: {
          name: task.workspace.name,
          path: workspace.path,
          git: workspace.git
            ? {
                branch: workspace.git.branch,
                commit: workspace.git.commit,
                dirty: isDirty,
              }
            : undefined,
        },
        metrics: {
          duration: Date.now() - startTime,
          promptTokens: response.tokens?.input,
          completionTokens: response.tokens?.output,
          toolCalls: response.toolCalls?.length || 0,
          streamEvents: publisher.getEventCount(),
        },
      });
    } catch (err) {
      const errorMessage = err instanceof Error ? err.message : String(err);

      await publisher.stream("error", { content: errorMessage });

      await publisher.result({
        status: "error",
        error: errorMessage,
        errorType: err instanceof Error ? err.constructor.name : "Unknown",
        session: {
          name: task.session.name,
          opencodeID: "",
          messageCount: 0,
        },
        workspace: {
          name: task.workspace.name,
          path: "",
        },
        metrics: {
          duration: Date.now() - startTime,
          toolCalls: 0,
          streamEvents: publisher.getEventCount(),
        },
      });
    }
  }

  private async executePrompt(
    sessionID: string,
    workspacePath: string,
    task: CodingTask,
    publisher: EventPublisher
  ): Promise<{
    text: string;
    tokens?: { input: number; output: number };
    toolCalls?: Array<{ name: string; output?: string }>;
  }> {
    const promptOptions: Parameters<OpencodeClient["session"]["prompt"]>[0] = {
      path: { id: sessionID },
      query: { directory: workspacePath },
      body: {
        parts: [{ type: "text" as const, text: task.prompt }],
      },
    };

    if (task.agent && promptOptions.body) {
      promptOptions.body.agent = task.agent;
    }

    if (task.model && promptOptions.body) {
      promptOptions.body.model = {
        providerID: task.model.providerID,
        modelID: task.model.modelID,
      };
    }

    let result: Awaited<ReturnType<OpencodeClient["session"]["prompt"]>>;
    try {
      result = await this.client.session.prompt(promptOptions);
    } catch (promptError) {
      const errorStr = String(promptError);
      const isSessionNotFound = 
        errorStr.includes("NotFoundError") ||
        errorStr.includes("Unexpected EOF") ||
        errorStr.includes("JSON Parse error");
      
      if (isSessionNotFound) {
        console.error(`[station-plugin] Session ${sessionID} not found - possible race condition:`, promptError);
        throw new Error(`Session ${sessionID} not found. OpenCode may not have persisted the session. Try again.`);
      }
      
      console.error("[station-plugin] Prompt threw exception:", promptError);
      throw promptError;
    }

    console.log("[station-plugin] Prompt result keys:", Object.keys(result || {}));

    if ("error" in result && result.error) {
      console.error("[station-plugin] Prompt returned error:", result.error);
      throw new Error(`Prompt failed: ${JSON.stringify(result.error)}`);
    }

    if (!result.data) {
      console.error("[station-plugin] Prompt returned no data, full result:", JSON.stringify(result));
      throw new Error("Prompt failed: no data in response");
    }

    const { parts } = result.data;

    let fullText = "";
    const toolCalls: Array<{ name: string; output?: string }> = [];

    for (const part of parts) {
      if (this.isTextPart(part)) {
        fullText += part.text;
        await publisher.stream("text", { content: part.text });
      } else if (this.isToolPart(part)) {
        if (part.state.status === "pending" || part.state.status === "running") {
          await publisher.stream("tool_start", {
            tool: {
              name: part.tool,
              callID: part.callID,
              args: part.state.input,
            },
          });
        } else if (part.state.status === "completed") {
          await publisher.stream("tool_end", {
            tool: {
              name: part.tool,
              callID: part.callID,
              output: part.state.output,
            },
          });
          toolCalls.push({ name: part.tool, output: part.state.output });
        } else if (part.state.status === "error") {
          await publisher.stream("tool_end", {
            tool: {
              name: part.tool,
              callID: part.callID,
              output: part.state.error,
            },
          });
          toolCalls.push({ name: part.tool, output: part.state.error });
        }
      }
    }

    return { text: fullText, toolCalls };
  }

  private isTextPart(part: Part): part is TextPart {
    return part.type === "text";
  }

  private isToolPart(part: Part): part is ToolPart {
    return part.type === "tool";
  }
}
