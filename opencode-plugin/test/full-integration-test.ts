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
  timeoutMs = 120000
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

async function testCompleteWorkflow(nc: Awaited<ReturnType<typeof connect>>) {
  console.log("\n=== Full Integration Test: Complete Workflow ===");

  const workspaceName = `full-test-${Date.now()}`;
  const sessionName = `session-${workspaceName}`;

  console.log("\n--- Step 1: Clone repo and analyze ---");
  const { events: events1, result: result1 } = await sendTask(nc, {
    workspace: {
      name: workspaceName,
      git: {
        url: "https://github.com/octocat/Hello-World.git",
        branch: "master",
      },
    },
    session: { name: sessionName },
    prompt: "List all files in this repository and describe what you see.",
  });

  if (result1.status !== "completed") {
    throw new Error(`Step 1 failed: ${result1.status} - ${result1.error}`);
  }
  console.log("  Step 1 completed");
  console.log(`     Workspace: ${result1.workspace.path}`);
  console.log(`     Git branch: ${result1.workspace.git?.branch}`);
  console.log(`     Message count: ${result1.session.messageCount}`);

  console.log("\n--- Step 2: Create a new file ---");
  const { events: events2, result: result2 } = await sendTask(nc, {
    workspace: { name: workspaceName },
    session: { name: sessionName, continue: true },
    prompt: "Create a file called notes.txt with the content 'Created by OpenCode integration test'. Use the bash tool with echo command.",
  });

  if (result2.status !== "completed") {
    throw new Error(`Step 2 failed: ${result2.status} - ${result2.error}`);
  }

  const hasToolEvents = events2.some(
    (e) => e.type === "tool_start" || e.type === "tool_end"
  );
  console.log("  Step 2 completed");
  console.log(`     Tool calls made: ${hasToolEvents}`);
  console.log(`     Message count: ${result2.session.messageCount}`);

  console.log("\n--- Step 3: Check if file was created (git dirty) ---");
  const { events: events3, result: result3 } = await sendTask(nc, {
    workspace: { name: workspaceName },
    session: { name: sessionName, continue: true },
    prompt: "Check if notes.txt exists and show its content. Use cat command.",
  });

  if (result3.status !== "completed") {
    throw new Error(`Step 3 failed: ${result3.status} - ${result3.error}`);
  }
  console.log("  Step 3 completed");
  console.log(`     Git dirty: ${result3.workspace.git?.dirty}`);
  console.log(`     Message count: ${result3.session.messageCount}`);

  console.log("\n--- Step 4: Session context verification ---");
  const { result: result4 } = await sendTask(nc, {
    workspace: { name: workspaceName },
    session: { name: sessionName, continue: true },
    prompt: "Summarize what we did in this session. What files did we look at and what file did we create?",
  });

  if (result4.status !== "completed") {
    throw new Error(`Step 4 failed: ${result4.status} - ${result4.error}`);
  }
  console.log("  Step 4 completed");
  console.log(`     Final message count: ${result4.session.messageCount}`);
  console.log(`     Session maintained context: ${result4.session.messageCount >= 4}`);

  console.log("\n--- Workflow Summary ---");
  console.log(`  Total messages: ${result4.session.messageCount}`);
  console.log(`  Workspace path: ${result4.workspace.path}`);
  console.log(`  Git dirty (files modified): ${result4.workspace.git?.dirty}`);
  console.log("  All steps completed successfully");
}

async function testErrorHandling(nc: Awaited<ReturnType<typeof connect>>) {
  console.log("\n=== Error Handling Test ===");

  const workspaceName = `error-test-${Date.now()}`;

  const { events, result } = await sendTask(nc, {
    workspace: {
      name: workspaceName,
      git: {
        url: "https://github.com/nonexistent-user-12345/nonexistent-repo-67890.git",
        branch: "main",
      },
    },
    session: { name: `session-${workspaceName}` },
    prompt: "List files",
  });

  const hasErrorEvent = events.some((e) => e.type === "error");

  if (result.status === "error") {
    console.log("  Error handled correctly");
    console.log(`     Status: ${result.status}`);
    console.log(`     Error type: ${result.error?.substring(0, 50)}...`);
    console.log(`     Error event emitted: ${hasErrorEvent}`);
  } else {
    console.log("  Warning: Expected error for nonexistent repo, got:", result.status);
  }
}

async function testMetrics(nc: Awaited<ReturnType<typeof connect>>) {
  console.log("\n=== Metrics Test ===");

  const workspaceName = `metrics-test-${Date.now()}`;

  const startTime = Date.now();
  const { events, result } = await sendTask(nc, {
    workspace: { name: workspaceName },
    session: { name: `session-${workspaceName}` },
    prompt: "What is 1 + 1? Answer briefly.",
  });
  const clientDuration = Date.now() - startTime;

  if (result.status !== "completed") {
    throw new Error(`Metrics test failed: ${result.status}`);
  }

  console.log("  Metrics captured");
  console.log(`     Server duration: ${result.metrics.duration}ms`);
  console.log(`     Client duration: ${clientDuration}ms`);
  console.log(`     Stream events: ${result.metrics.streamEvents}`);
  console.log(`     Tool calls: ${result.metrics.toolCalls}`);
  console.log(`     Events received: ${events.length}`);

  if (result.metrics.duration <= 0) {
    throw new Error("Expected positive duration in metrics");
  }
  if (result.metrics.streamEvents !== events.length) {
    console.log(`  Warning: Event count mismatch - server: ${result.metrics.streamEvents}, client: ${events.length}`);
  }
}

async function runTests() {
  console.log("=== Full Integration Tests ===");
  console.log(`NATS URL: ${process.env.NATS_URL || "nats://localhost:4222"}`);

  const nc = await connect({
    servers: process.env.NATS_URL || "nats://localhost:4222",
  });
  console.log("Connected to NATS");

  let passed = 0;
  let failed = 0;

  try {
    await testCompleteWorkflow(nc);
    passed++;
  } catch (err) {
    console.error("  Complete workflow test failed:", err);
    failed++;
  }

  try {
    await testErrorHandling(nc);
    passed++;
  } catch (err) {
    console.error("  Error handling test failed:", err);
    failed++;
  }

  try {
    await testMetrics(nc);
    passed++;
  } catch (err) {
    console.error("  Metrics test failed:", err);
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
