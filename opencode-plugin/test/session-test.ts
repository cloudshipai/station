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

async function test1_SessionCreation(nc: Awaited<ReturnType<typeof connect>>) {
  console.log("\n=== Test 1: Session Creation ===");

  const sessionName = `session-${Date.now()}`;
  const workspaceName = `workspace-${Date.now()}`;

  const { events, result } = await sendTask(nc, {
    workspace: { name: workspaceName },
    session: { name: sessionName },
    prompt: "What is 2 + 2? Answer with just the number.",
  });

  const eventTypes = events.map((e) => e.type);

  if (!eventTypes.includes("session_created")) {
    throw new Error("Expected session_created event");
  }

  if (result.status !== "completed") {
    throw new Error(`Expected completed status, got: ${result.status}`);
  }

  if (result.session.name !== sessionName) {
    throw new Error(`Expected session name ${sessionName}, got: ${result.session.name}`);
  }

  if (!result.session.opencodeID) {
    throw new Error("Expected opencodeID in session");
  }

  if (result.session.messageCount !== 1) {
    throw new Error(`Expected messageCount 1, got: ${result.session.messageCount}`);
  }

  console.log("  Session created successfully");
  console.log(`     Name: ${result.session.name}`);
  console.log(`     OpenCode ID: ${result.session.opencodeID}`);
  console.log(`     Message count: ${result.session.messageCount}`);

  return { sessionName, workspaceName, opencodeID: result.session.opencodeID };
}

async function test2_SessionContinuation(
  nc: Awaited<ReturnType<typeof connect>>,
  sessionName: string,
  workspaceName: string,
  expectedOpencodeID: string
) {
  console.log("\n=== Test 2: Session Continuation ===");

  const { events, result } = await sendTask(nc, {
    workspace: { name: workspaceName },
    session: { name: sessionName, continue: true },
    prompt: "What was my previous question? Summarize it briefly.",
  });

  const eventTypes = events.map((e) => e.type);

  if (eventTypes.includes("session_created")) {
    throw new Error("Should have reused session, not created new one");
  }
  if (!eventTypes.includes("session_reused")) {
    throw new Error("Expected session_reused event");
  }

  if (result.status !== "completed") {
    throw new Error(`Expected completed status, got: ${result.status}`);
  }

  if (result.session.opencodeID !== expectedOpencodeID) {
    throw new Error(
      `Expected same opencodeID ${expectedOpencodeID}, got: ${result.session.opencodeID}`
    );
  }

  if (result.session.messageCount !== 2) {
    throw new Error(`Expected messageCount 2, got: ${result.session.messageCount}`);
  }

  console.log("  Session continued successfully");
  console.log(`     OpenCode ID: ${result.session.opencodeID} (same as before)`);
  console.log(`     Message count: ${result.session.messageCount}`);

  return result.session.messageCount;
}

async function test3_MultipleMessages(
  nc: Awaited<ReturnType<typeof connect>>,
  sessionName: string,
  workspaceName: string
) {
  console.log("\n=== Test 3: Multiple Messages in Session ===");

  const prompts = [
    "What is 5 + 5? Answer with just the number.",
    "What is 10 + 10? Answer with just the number.",
    "What is 20 + 20? Answer with just the number.",
  ];

  let lastMessageCount = 2;

  for (let i = 0; i < prompts.length; i++) {
    console.log(`  Sending message ${i + 3}...`);

    const { result } = await sendTask(nc, {
      workspace: { name: workspaceName },
      session: { name: sessionName, continue: true },
      prompt: prompts[i],
    });

    if (result.status !== "completed") {
      throw new Error(`Message ${i + 3} failed: ${result.status}`);
    }

    const expectedCount = lastMessageCount + 1;
    if (result.session.messageCount !== expectedCount) {
      throw new Error(
        `Expected messageCount ${expectedCount}, got: ${result.session.messageCount}`
      );
    }

    lastMessageCount = result.session.messageCount;
    console.log(`     Message count: ${result.session.messageCount}`);
  }

  console.log("  Multiple messages handled correctly");
  console.log(`     Final message count: ${lastMessageCount}`);
}

async function test4_NewSessionWithSameName(
  nc: Awaited<ReturnType<typeof connect>>,
  sessionName: string,
  workspaceName: string
) {
  console.log("\n=== Test 4: New Session (continue=false) ===");

  const { events, result } = await sendTask(nc, {
    workspace: { name: workspaceName },
    session: { name: sessionName, continue: false },
    prompt: "Start fresh. What is 1 + 1? Answer with just the number.",
  });

  const eventTypes = events.map((e) => e.type);

  if (!eventTypes.includes("session_created")) {
    throw new Error("Expected session_created event when continue=false");
  }

  if (result.status !== "completed") {
    throw new Error(`Expected completed status, got: ${result.status}`);
  }

  if (result.session.messageCount !== 1) {
    throw new Error(`Expected messageCount 1 for new session, got: ${result.session.messageCount}`);
  }

  console.log("  New session created with same name");
  console.log(`     Message count reset to: ${result.session.messageCount}`);
}

async function runTests() {
  console.log("=== Session Continuation Tests ===");
  console.log(`NATS URL: ${process.env.NATS_URL || "nats://localhost:4222"}`);

  const nc = await connect({
    servers: process.env.NATS_URL || "nats://localhost:4222",
  });
  console.log("Connected to NATS");

  let passed = 0;
  let failed = 0;

  let sessionInfo: { sessionName: string; workspaceName: string; opencodeID: string } | null = null;

  try {
    sessionInfo = await test1_SessionCreation(nc);
    passed++;
  } catch (err) {
    console.error("  Test 1 failed:", err);
    failed++;
  }

  if (sessionInfo) {
    try {
      await test2_SessionContinuation(
        nc,
        sessionInfo.sessionName,
        sessionInfo.workspaceName,
        sessionInfo.opencodeID
      );
      passed++;
    } catch (err) {
      console.error("  Test 2 failed:", err);
      failed++;
    }

    try {
      await test3_MultipleMessages(nc, sessionInfo.sessionName, sessionInfo.workspaceName);
      passed++;
    } catch (err) {
      console.error("  Test 3 failed:", err);
      failed++;
    }

    try {
      await test4_NewSessionWithSameName(nc, sessionInfo.sessionName, sessionInfo.workspaceName);
      passed++;
    } catch (err) {
      console.error("  Test 4 failed:", err);
      failed++;
    }
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
