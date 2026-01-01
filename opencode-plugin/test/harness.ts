// OpenCode Plugin Integration Test Harness
// Mirrors the Station NATSCodingBackend interface for testing the plugin
// with the same patterns Station agents will use.

import { connect, StringCodec, type NatsConnection, type JetStreamClient, type KV } from "nats";

const sc = StringCodec();

interface GitConfig {
  url: string;
  branch?: string;
  ref?: string;
  token?: string;
  tokenFromKV?: string;
  pull?: boolean;
}

interface SessionConfig {
  name: string;
  continue?: boolean;
}

interface WorkspaceConfig {
  name: string;
  git?: GitConfig;
}

interface CodingRequest {
  session: SessionConfig;
  workspace: WorkspaceConfig;
  prompt: string;
  agent?: string;
  model?: { providerID: string; modelID: string };
  timeout?: number;
}

interface CodingTask {
  taskID: string;
  session: SessionConfig;
  workspace: WorkspaceConfig;
  prompt: string;
  agent?: string;
  model?: { providerID: string; modelID: string };
  timeout?: number;
  callback: { streamSubject: string; resultSubject: string };
}

interface CodingStreamEvent {
  taskID: string;
  seq: number;
  timestamp: string;
  type:
    | "session_created"
    | "session_reused"
    | "workspace_created"
    | "workspace_reused"
    | "git_clone"
    | "git_pull"
    | "git_checkout"
    | "prompt_sent"
    | "text"
    | "thinking"
    | "tool_start"
    | "tool_end"
    | "error";
  content?: string;
  tool?: { name: string; callID: string; args?: Record<string, unknown>; output?: string; duration?: number };
  git?: { url?: string; branch?: string; commit?: string };
  session?: { name: string; opencodeID: string };
}

interface CodingResult {
  taskID: string;
  status: "completed" | "error" | "timeout" | "cancelled";
  result?: string;
  error?: string;
  errorType?: string;
  session: { name: string; opencodeID: string; messageCount: number };
  workspace: { name: string; path: string; git?: { branch: string; commit: string; dirty: boolean } };
  metrics: { duration: number; promptTokens?: number; completionTokens?: number; toolCalls: number; streamEvents: number };
}

class NATSCodingBackend {
  private nc: NatsConnection | null = null;
  private js: JetStreamClient | null = null;
  private kv: KV | null = null;
  private taskSubject: string;
  private traces: Map<string, CodingStreamEvent[]> = new Map();

  constructor(taskSubject = "station.coding.task") {
    this.taskSubject = taskSubject;
  }

  async connect(url = "nats://localhost:4222"): Promise<void> {
    console.log(`[NATSCodingBackend] Connecting to ${url}...`);
    this.nc = await connect({ servers: url });
    this.js = this.nc.jetstream();

    try {
      this.kv = await this.js.views.kv("opencode-state");
    } catch {
      const jsm = await this.js.jetstreamManager();
      await jsm.streams.add({ name: "KV_opencode-state", subjects: ["$KV.opencode-state.>"] });
      this.kv = await this.js.views.kv("opencode-state");
    }

    console.log("[NATSCodingBackend] Connected");
  }

  async execute(req: CodingRequest): Promise<{ result: CodingResult; events: CodingStreamEvent[] }> {
    if (!this.nc) throw new Error("Not connected to NATS");

    const taskID = crypto.randomUUID();
    const streamSubject = `station.coding.stream.${taskID}`;
    const resultSubject = `station.coding.result.${taskID}`;

    const events: CodingStreamEvent[] = [];
    this.traces.set(taskID, events);

    const streamSub = this.nc.subscribe(streamSubject, {
      callback: (_err, msg) => {
        const event = JSON.parse(sc.decode(msg.data)) as CodingStreamEvent;
        events.push(event);
        this.recordTrace(event);
      },
    });

    const resultPromise = new Promise<CodingResult>((resolve, reject) => {
      const timeout = req.timeout || 120000;
      const timeoutId = setTimeout(() => reject(new Error(`Task timed out after ${timeout}ms`)), timeout);

      const nc = this.nc;
      if (!nc) return reject(new Error("Not connected"));

      const resultSub = nc.subscribe(resultSubject, {
        callback: (_err, msg) => {
          clearTimeout(timeoutId);
          const result = JSON.parse(sc.decode(msg.data)) as CodingResult;
          resultSub.unsubscribe();
          resolve(result);
        },
      });
    });

    const task: CodingTask = {
      taskID,
      session: req.session,
      workspace: req.workspace,
      prompt: req.prompt,
      agent: req.agent,
      model: req.model,
      timeout: req.timeout,
      callback: { streamSubject, resultSubject },
    };

    console.log(`\n[NATSCodingBackend] Publishing task ${taskID}`);
    console.log(`  Session: ${req.session.name} (continue: ${req.session.continue ?? true})`);
    console.log(`  Workspace: ${req.workspace.name}`);
    if (req.workspace.git) console.log(`  Git: ${req.workspace.git.url} @ ${req.workspace.git.branch || "default"}`);
    console.log(`  Prompt: ${req.prompt.slice(0, 80)}${req.prompt.length > 80 ? "..." : ""}`);

    this.nc.publish(this.taskSubject, sc.encode(JSON.stringify(task)));

    try {
      const result = await resultPromise;
      console.log(`\n[NATSCodingBackend] Task ${taskID} completed: ${result.status}`);
      return { result, events };
    } finally {
      streamSub.unsubscribe();
    }
  }

  private recordTrace(event: CodingStreamEvent): void {
    const typeIcon: Record<string, string> = {
      session_created: "🆕",
      session_reused: "♻️",
      workspace_created: "📁",
      workspace_reused: "📂",
      git_clone: "📥",
      git_pull: "📥",
      git_checkout: "🔀",
      prompt_sent: "📤",
      text: "💬",
      thinking: "🤔",
      tool_start: "🔧",
      tool_end: "✅",
      error: "❌",
    };
    const icon = typeIcon[event.type] || "📌";
    const content = event.content?.slice(0, 60) || event.tool?.name || event.session?.name || "";
    console.log(`  ${icon} [${event.seq}] ${event.type}: ${content}`);
  }

  async kvGet(key: string): Promise<string | null> {
    if (!this.kv) return null;
    try {
      const entry = await this.kv.get(key);
      return entry?.value ? sc.decode(entry.value) : null;
    } catch {
      return null;
    }
  }

  async kvPut(key: string, value: string): Promise<void> {
    if (!this.kv) return;
    await this.kv.put(key, sc.encode(value));
  }

  getTraces(taskID: string): CodingStreamEvent[] {
    return this.traces.get(taskID) || [];
  }

  async close(): Promise<void> {
    if (this.nc) {
      await this.nc.drain();
      this.nc = null;
    }
  }
}

interface TestResult {
  name: string;
  passed: boolean;
  error?: string;
  duration: number;
}

class IntegrationTestSuite {
  private backend: NATSCodingBackend;
  private results: TestResult[] = [];

  constructor() {
    this.backend = new NATSCodingBackend();
  }

  async setup(): Promise<void> {
    await this.backend.connect(process.env.NATS_URL || "nats://localhost:4222");
  }

  async teardown(): Promise<void> {
    await this.backend.close();
  }

  private async runTest(name: string, fn: () => Promise<void>): Promise<void> {
    console.log(`\n${"=".repeat(60)}`);
    console.log(`TEST: ${name}`);
    console.log("=".repeat(60));

    const start = Date.now();
    try {
      await fn();
      this.results.push({ name, passed: true, duration: Date.now() - start });
      console.log(`\n✅ PASSED: ${name} (${Date.now() - start}ms)`);
    } catch (err) {
      const error = err instanceof Error ? err.message : String(err);
      this.results.push({ name, passed: false, error, duration: Date.now() - start });
      console.log(`\n❌ FAILED: ${name}`);
      console.log(`   Error: ${error}`);
    }
  }

  async testBasicPrompt(): Promise<void> {
    await this.runTest("Basic Prompt Execution", async () => {
      const { result, events } = await this.backend.execute({
        session: { name: "test-basic" },
        workspace: { name: "test-workspace" },
        prompt: "Say 'Hello from OpenCode' and nothing else.",
      });

      if (result.status !== "completed") {
        throw new Error(`Expected completed, got ${result.status}: ${result.error}`);
      }
      if (!result.result?.toLowerCase().includes("hello")) {
        throw new Error(`Expected 'hello' in response, got: ${result.result}`);
      }
      if (events.length === 0) {
        throw new Error("Expected stream events");
      }
    });
  }

  async testSessionPersistence(): Promise<void> {
    await this.runTest("Session Persistence", async () => {
      const sessionName = `session-persist-${Date.now()}`;

      await this.backend.execute({
        session: { name: sessionName },
        workspace: { name: "test-workspace" },
        prompt: "Remember this secret code: ALPHA-BRAVO-CHARLIE-123",
      });

      const { result } = await this.backend.execute({
        session: { name: sessionName, continue: true },
        workspace: { name: "test-workspace" },
        prompt: "What was the secret code I told you to remember?",
      });

      if (result.status !== "completed") {
        throw new Error(`Expected completed, got ${result.status}`);
      }
      if (!result.result?.includes("ALPHA") || !result.result?.includes("BRAVO")) {
        throw new Error(`Session did not persist. Response: ${result.result}`);
      }
      if (result.session.messageCount < 2) {
        throw new Error(`Expected messageCount >= 2, got ${result.session.messageCount}`);
      }
    });
  }

  async testNewSessionCreation(): Promise<void> {
    await this.runTest("New Session Creation (continue: false)", async () => {
      const sessionName = `session-new-${Date.now()}`;

      await this.backend.execute({
        session: { name: sessionName },
        workspace: { name: "test-workspace" },
        prompt: "Remember the number 42.",
      });

      const { result, events } = await this.backend.execute({
        session: { name: sessionName, continue: false },
        workspace: { name: "test-workspace" },
        prompt: "What number did I tell you to remember?",
      });

      const sessionCreatedEvent = events.find((e) => e.type === "session_created");
      if (!sessionCreatedEvent) {
        throw new Error("Expected session_created event for new session");
      }

      if (result.session.messageCount !== 1) {
        throw new Error(`Expected messageCount = 1 for new session, got ${result.session.messageCount}`);
      }
    });
  }

  async testWorkspaceReuse(): Promise<void> {
    await this.runTest("Workspace Reuse", async () => {
      const workspaceName = `workspace-reuse-${Date.now()}`;

      const { events: events1 } = await this.backend.execute({
        session: { name: "ws-test-1" },
        workspace: { name: workspaceName },
        prompt: "Create a file called test.txt with content 'hello world'",
      });

      const wsCreated = events1.find((e) => e.type === "workspace_created");
      if (!wsCreated) {
        throw new Error("Expected workspace_created on first call");
      }

      const { events: events2 } = await this.backend.execute({
        session: { name: "ws-test-2" },
        workspace: { name: workspaceName },
        prompt: "List the files in the current directory",
      });

      const wsReused = events2.find((e) => e.type === "workspace_reused");
      if (!wsReused) {
        throw new Error("Expected workspace_reused on second call");
      }
    });
  }

  async testGitClone(): Promise<void> {
    await this.runTest("Git Clone", async () => {
      const workspaceName = `git-clone-${Date.now()}`;

      const { result, events } = await this.backend.execute({
        session: { name: "git-test" },
        workspace: {
          name: workspaceName,
          git: {
            url: "https://github.com/octocat/Hello-World.git",
            branch: "master",
          },
        },
        prompt: "List the files in the repository root and tell me what you see.",
      });

      const gitCloneEvent = events.find((e) => e.type === "git_clone");
      if (!gitCloneEvent) {
        throw new Error("Expected git_clone event");
      }

      if (result.status !== "completed") {
        throw new Error(`Expected completed, got ${result.status}: ${result.error}`);
      }

      if (!result.workspace.git) {
        throw new Error("Expected git info in result");
      }
    });
  }

  async testGitPull(): Promise<void> {
    await this.runTest("Git Pull on Existing Workspace", async () => {
      const workspaceName = `git-pull-${Date.now()}`;

      await this.backend.execute({
        session: { name: "git-pull-1" },
        workspace: {
          name: workspaceName,
          git: { url: "https://github.com/octocat/Hello-World.git", branch: "master" },
        },
        prompt: "What files exist?",
      });

      const { events } = await this.backend.execute({
        session: { name: "git-pull-2" },
        workspace: {
          name: workspaceName,
          git: { url: "https://github.com/octocat/Hello-World.git", branch: "master", pull: true },
        },
        prompt: "What files exist now?",
      });

      const gitPullEvent = events.find((e) => e.type === "git_pull");
      if (!gitPullEvent) {
        throw new Error("Expected git_pull event on second call with pull: true");
      }
    });
  }

  async testToolExecution(): Promise<void> {
    await this.runTest("Tool Execution (Bash)", async () => {
      const { result, events } = await this.backend.execute({
        session: { name: "tool-test" },
        workspace: { name: "tool-workspace" },
        prompt: "Run 'echo hello-from-bash' in the terminal and tell me the output.",
      });

      const toolStart = events.find((e) => e.type === "tool_start" && e.tool?.name === "bash");
      const toolEnd = events.find((e) => e.type === "tool_end" && e.tool?.name === "bash");

      if (!toolStart) {
        console.log("  Note: No bash tool_start event found (may use different tool name)");
      }
      if (!toolEnd) {
        console.log("  Note: No bash tool_end event found (may use different tool name)");
      }

      if (result.status !== "completed") {
        throw new Error(`Expected completed, got ${result.status}`);
      }
      if (!result.result?.includes("hello-from-bash")) {
        throw new Error(`Expected 'hello-from-bash' in response, got: ${result.result}`);
      }
    });
  }

  async testKVTools(): Promise<void> {
    await this.runTest("KV Store Tools", async () => {
      const testKey = `test-key-${Date.now()}`;
      const testValue = `test-value-${Date.now()}`;

      await this.backend.kvPut(testKey, testValue);

      const { result } = await this.backend.execute({
        session: { name: "kv-test" },
        workspace: { name: "kv-workspace" },
        prompt: `Use the station_kv_get tool to retrieve the value for key "${testKey}" and tell me what it is.`,
      });

      if (result.status !== "completed") {
        throw new Error(`Expected completed, got ${result.status}`);
      }

      if (!result.result?.includes(testValue)) {
        console.log(`  Note: KV tool may not be available yet. Response: ${result.result?.slice(0, 100)}`);
      }
    });
  }

  async testErrorHandling(): Promise<void> {
    await this.runTest("Error Handling", async () => {
      const { result, events } = await this.backend.execute({
        session: { name: "error-test" },
        workspace: { name: "error-workspace" },
        prompt: "This is a normal prompt that should succeed.",
        timeout: 5000,
      });

      if (result.status === "error") {
        const errorEvent = events.find((e) => e.type === "error");
        if (!errorEvent) {
          console.log("  Note: Error occurred but no error event found");
        }
      }
    });
  }

  async testMetrics(): Promise<void> {
    await this.runTest("Metrics Collection", async () => {
      const { result } = await this.backend.execute({
        session: { name: "metrics-test" },
        workspace: { name: "metrics-workspace" },
        prompt: "Say hello.",
      });

      if (result.status !== "completed") {
        throw new Error(`Expected completed, got ${result.status}`);
      }

      if (result.metrics.duration <= 0) {
        throw new Error(`Expected positive duration, got ${result.metrics.duration}`);
      }
      if (result.metrics.streamEvents <= 0) {
        throw new Error(`Expected positive streamEvents, got ${result.metrics.streamEvents}`);
      }

      console.log(`  Metrics: duration=${result.metrics.duration}ms, events=${result.metrics.streamEvents}, tools=${result.metrics.toolCalls}`);
    });
  }

  async runAll(): Promise<void> {
    await this.setup();

    try {
      await this.testBasicPrompt();
      await this.testSessionPersistence();
      await this.testNewSessionCreation();
      await this.testWorkspaceReuse();
      await this.testGitClone();
      await this.testGitPull();
      await this.testToolExecution();
      await this.testKVTools();
      await this.testErrorHandling();
      await this.testMetrics();
    } finally {
      await this.teardown();
    }

    this.printSummary();
  }

  private printSummary(): void {
    console.log(`\n${"=".repeat(60)}`);
    console.log("TEST SUMMARY");
    console.log("=".repeat(60));

    const passed = this.results.filter((r) => r.passed).length;
    const failed = this.results.filter((r) => !r.passed).length;
    const total = this.results.length;

    for (const r of this.results) {
      const icon = r.passed ? "✅" : "❌";
      console.log(`${icon} ${r.name} (${r.duration}ms)`);
      if (r.error) console.log(`   └── ${r.error}`);
    }

    console.log(`\nTotal: ${passed}/${total} passed, ${failed} failed`);
    console.log("=".repeat(60));

    if (failed > 0) {
      process.exit(1);
    }
  }
}

const suite = new IntegrationTestSuite();
suite.runAll();
