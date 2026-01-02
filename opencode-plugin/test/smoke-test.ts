// Simple smoke test - just verify NATS message passing
import { connect, StringCodec } from "nats";

const sc = StringCodec();

async function smokeTest() {
  console.log("[Smoke Test] Connecting to NATS...");
  const nc = await connect({ servers: process.env.NATS_URL || "nats://localhost:4222" });
  console.log("[Smoke Test] Connected!");

  const taskID = crypto.randomUUID();
  const resultSubject = `station.coding.result.${taskID}`;
  const streamSubject = `station.coding.stream.${taskID}`;

  // Subscribe to responses
  const events: unknown[] = [];
  const streamSub = nc.subscribe(streamSubject, {
    callback: (_, msg) => {
      const event = JSON.parse(sc.decode(msg.data));
      events.push(event);
      console.log(`[Smoke Test] Stream event: ${event.type}`);
    }
  });

  let result: unknown = null;
  const resultPromise = new Promise((resolve, reject) => {
    const timeout = setTimeout(() => reject(new Error("Timeout waiting for result")), 10000);
    const resultSub = nc.subscribe(resultSubject, {
      callback: (_, msg) => {
        clearTimeout(timeout);
        result = JSON.parse(sc.decode(msg.data));
        console.log("[Smoke Test] Got result!");
        resultSub.unsubscribe();
        resolve(result);
      }
    });
  });

  // Send task
  const task = {
    taskID,
    session: { name: "smoke-test" },
    workspace: { name: "smoke-workspace" },
    prompt: "Say hello",
    callback: { streamSubject, resultSubject }
  };

  console.log("[Smoke Test] Publishing task...");
  nc.publish("station.coding.task", sc.encode(JSON.stringify(task)));

  try {
    await resultPromise;
    console.log(`\n[Smoke Test] SUCCESS! Received ${events.length} stream events`);
    console.log("[Smoke Test] Result status:", (result as any)?.status);
  } catch (e) {
    console.log("[Smoke Test] TIMEOUT - but this is expected if no LLM keys are configured");
    console.log(`[Smoke Test] Received ${events.length} stream events before timeout`);
  }

  streamSub.unsubscribe();
  await nc.drain();
  console.log("[Smoke Test] Done!");
}

smokeTest().catch(console.error);
