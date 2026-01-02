import { connect, StringCodec } from "nats";

const sc = StringCodec();

interface StreamEvent {
  type: string;
  taskID: string;
  timestamp: string;
  payload: unknown;
}

interface CodingResult {
  taskID: string;
  status: "completed" | "error" | "timeout";
  result?: string;
  error?: string;
  workspace: {
    name: string;
    path: string;
    git?: {
      branch: string;
      commit: string;
      dirty: boolean;
    };
  };
  session: {
    name: string;
    opencodeID: string;
    messageCount: number;
  };
  metrics: {
    duration: number;
    toolCalls: number;
    streamEvents: number;
  };
}

async function sendTask(
  nc: Awaited<ReturnType<typeof connect>>,
  task: {
    workspace: { name: string; git?: { url: string; branch?: string } };
    session: { name: string; continue?: boolean };
    prompt: string;
  },
  timeoutMs = 60000
): Promise<{ events: StreamEvent[]; result: CodingResult }> {
  const taskID = crypto.randomUUID();
  const streamSubject = `station.coding.stream.${taskID}`;
  const resultSubject = `station.coding.result.${taskID}`;

  const events: StreamEvent[] = [];

  const streamSub = nc.subscribe(streamSubject, {
    callback: (_, msg) => {
      const event = JSON.parse(sc.decode(msg.data)) as StreamEvent;
      events.push(event);
      console.log(`  [stream] ${event.type}`);
    },
  });

  const resultPromise = new Promise<CodingResult>((resolve, reject) => {
    const timeout = setTimeout(() => {
      reject(new Error(`Timeout waiting for result (${timeoutMs}ms)`));
    }, timeoutMs);

    nc.subscribe(resultSubject, {
      callback: (_, msg) => {
        clearTimeout(timeout);
        const result = JSON.parse(sc.decode(msg.data)) as CodingResult;
        resolve(result);
      },
    });
  });

  const fullTask = {
    taskID,
    ...task,
    callback: { streamSubject, resultSubject },
  };

  nc.publish("station.coding.task", sc.encode(JSON.stringify(fullTask)));

  const result = await resultPromise;
  streamSub.unsubscribe();

  return { events, result };
}

async function test1_GitClone(nc: Awaited<ReturnType<typeof connect>>) {
  console.log("\n=== Test 1: Git Clone from Public Repo ===");

  const workspaceName = `git-test-${Date.now()}`;

  const { events, result } = await sendTask(nc, {
    workspace: {
      name: workspaceName,
      git: {
        url: "https://github.com/octocat/Hello-World.git",
        branch: "master",
      },
    },
    session: { name: `session-${workspaceName}` },
    prompt: "List all files in this repository using ls -la. Just show the file listing.",
  });

  const eventTypes = events.map((e) => e.type);
  console.log(`  Events received: ${eventTypes.join(", ")}`);

  if (!eventTypes.includes("workspace_created")) {
    throw new Error("Expected workspace_created event");
  }
  if (!eventTypes.includes("git_clone")) {
    throw new Error("Expected git_clone event");
  }
  if (!eventTypes.includes("session_created")) {
    throw new Error("Expected session_created event");
  }

  if (result.status !== "completed") {
    throw new Error(`Expected completed status, got: ${result.status} - ${result.error}`);
  }
  if (!result.workspace.git) {
    throw new Error("Expected git info in workspace");
  }
  if (result.workspace.git.branch !== "master") {
    throw new Error(`Expected branch 'master', got: ${result.workspace.git.branch}`);
  }
  if (!result.workspace.git.commit) {
    throw new Error("Expected commit hash in git info");
  }

  console.log("  Git clone successful");
  console.log(`     Branch: ${result.workspace.git.branch}`);
  console.log(`     Commit: ${result.workspace.git.commit.substring(0, 8)}`);
  console.log(`     Dirty: ${result.workspace.git.dirty}`);

  return workspaceName;
}

async function test2_WorkspaceReuse(
  nc: Awaited<ReturnType<typeof connect>>,
  workspaceName: string
) {
  console.log("\n=== Test 2: Workspace Reuse (Git Pull) ===");

  const { events, result } = await sendTask(nc, {
    workspace: {
      name: workspaceName,
      git: {
        url: "https://github.com/octocat/Hello-World.git",
        branch: "master",
      },
    },
    session: { name: `session-${workspaceName}`, continue: true },
    prompt: "What branch are we on? Use git branch --show-current",
  });

  const eventTypes = events.map((e) => e.type);
  console.log(`  Events received: ${eventTypes.join(", ")}`);

  if (eventTypes.includes("workspace_created")) {
    throw new Error("Should have reused workspace, not created new one");
  }
  if (!eventTypes.includes("workspace_reused")) {
    throw new Error("Expected workspace_reused event");
  }
  if (!eventTypes.includes("git_pull")) {
    throw new Error("Expected git_pull event");
  }

  if (result.status !== "completed") {
    throw new Error(`Expected completed status, got: ${result.status}`);
  }

  console.log("  Workspace reused with git pull");
  console.log(`     Session messages: ${result.session.messageCount}`);
}

async function test3_BranchCheckout(nc: Awaited<ReturnType<typeof connect>>) {
  console.log("\n=== Test 3: Branch Checkout ===");

  const workspaceName = `branch-test-${Date.now()}`;

  const { events, result } = await sendTask(nc, {
    workspace: {
      name: workspaceName,
      git: {
        url: "https://github.com/octocat/Hello-World.git",
        branch: "test",
      },
    },
    session: { name: `session-${workspaceName}` },
    prompt: "What branch is checked out? Use git branch --show-current",
  });

  if (result.status !== "completed") {
    throw new Error(`Expected completed status, got: ${result.status} - ${result.error}`);
  }

  if (!result.workspace.git) {
    throw new Error("Expected git info in workspace");
  }

  console.log("  Branch checkout attempted");
  console.log("     Requested: test");
  console.log(`     Actual: ${result.workspace.git.branch}`);
}

async function test4_NoGitWorkspace(nc: Awaited<ReturnType<typeof connect>>) {
  console.log("\n=== Test 4: Workspace Without Git ===");

  const workspaceName = `no-git-${Date.now()}`;

  const { events, result } = await sendTask(nc, {
    workspace: { name: workspaceName },
    session: { name: `session-${workspaceName}` },
    prompt: "Create a file called test.txt with content 'hello world'. Use echo command.",
  });

  const eventTypes = events.map((e) => e.type);

  if (!eventTypes.includes("workspace_created")) {
    throw new Error("Expected workspace_created event");
  }
  if (eventTypes.includes("git_clone") || eventTypes.includes("git_pull")) {
    throw new Error("Should not have git events for non-git workspace");
  }

  if (result.status !== "completed") {
    throw new Error(`Expected completed status, got: ${result.status}`);
  }

  if (result.workspace.git) {
    throw new Error("Should not have git info for non-git workspace");
  }

  console.log("  Non-git workspace created");
  console.log(`     Path: ${result.workspace.path}`);
}

async function runTests() {
  console.log("=== Git Workspace Integration Tests ===");
  console.log(`NATS URL: ${process.env.NATS_URL || "nats://localhost:4222"}`);

  const nc = await connect({
    servers: process.env.NATS_URL || "nats://localhost:4222",
  });
  console.log("Connected to NATS");

  const opencodeUrl = process.env.OPENCODE_URL || "http://localhost:4097";
  console.log(`Bootstrapping OpenCode at ${opencodeUrl}...`);
  await fetch(`${opencodeUrl}/config?directory=/workspaces`);
  await new Promise((r) => setTimeout(r, 2000));
  console.log("OpenCode bootstrapped");

  let passed = 0;
  let failed = 0;

  try {
    const workspaceName = await test1_GitClone(nc);
    passed++;

    await test2_WorkspaceReuse(nc, workspaceName);
    passed++;
  } catch (err) {
    console.error("  Test 1/2 failed:", err);
    failed++;
  }

  try {
    await test3_BranchCheckout(nc);
    passed++;
  } catch (err) {
    console.error("  Test 3 failed:", err);
    failed++;
  }

  try {
    await test4_NoGitWorkspace(nc);
    passed++;
  } catch (err) {
    console.error("  Test 4 failed:", err);
    failed++;
  }

  await nc.drain();

  console.log("\n=== Results ===");
  console.log(`Passed: ${passed}`);
  console.log(`Failed: ${failed}`);

  if (failed > 0) {
    process.exit(1);
  }
}

runTests().catch((err) => {
  console.error("Fatal error:", err);
  process.exit(1);
});
