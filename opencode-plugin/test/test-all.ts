import { spawn } from "node:child_process";
import { join } from "node:path";

interface TestResult {
  name: string;
  passed: boolean;
  duration: number;
  output: string;
}

async function runTest(testFile: string): Promise<TestResult> {
  const startTime = Date.now();
  const testPath = join(import.meta.dir, testFile);

  return new Promise((resolve) => {
    const proc = spawn("bun", ["run", testPath], {
      env: { ...process.env },
      stdio: ["inherit", "pipe", "pipe"],
    });

    let output = "";

    proc.stdout?.on("data", (data) => {
      const text = data.toString();
      output += text;
      process.stdout.write(text);
    });

    proc.stderr?.on("data", (data) => {
      const text = data.toString();
      output += text;
      process.stderr.write(text);
    });

    proc.on("close", (code) => {
      resolve({
        name: testFile,
        passed: code === 0,
        duration: Date.now() - startTime,
        output,
      });
    });

    proc.on("error", (err) => {
      resolve({
        name: testFile,
        passed: false,
        duration: Date.now() - startTime,
        output: `Error spawning test: ${err.message}`,
      });
    });
  });
}

async function main() {
  console.log("╔════════════════════════════════════════════════════════════╗");
  console.log("║          Station OpenCode Plugin - Test Suite              ║");
  console.log("╚════════════════════════════════════════════════════════════╝");
  console.log();
  console.log(`NATS URL: ${process.env.NATS_URL || "nats://localhost:4222"}`);
  console.log(`Time: ${new Date().toISOString()}`);
  console.log();

  const tests = [
    "smoke-test.ts",
    "git-workspace-test.ts",
    "session-test.ts",
    "full-integration-test.ts",
  ];

  const results: TestResult[] = [];
  const suiteStart = Date.now();

  for (const test of tests) {
    console.log("┌────────────────────────────────────────────────────────────┐");
    console.log(`│ Running: ${test.padEnd(50)}│`);
    console.log("└────────────────────────────────────────────────────────────┘");

    const result = await runTest(test);
    results.push(result);

    console.log();
    if (result.passed) {
      console.log(`✅ ${test} PASSED (${result.duration}ms)`);
    } else {
      console.log(`❌ ${test} FAILED (${result.duration}ms)`);
    }
    console.log();
  }

  const suiteDuration = Date.now() - suiteStart;
  const passed = results.filter((r) => r.passed).length;
  const failed = results.filter((r) => !r.passed).length;

  console.log("╔════════════════════════════════════════════════════════════╗");
  console.log("║                     Test Suite Results                     ║");
  console.log("╠════════════════════════════════════════════════════════════╣");

  for (const result of results) {
    const status = result.passed ? "✅ PASS" : "❌ FAIL";
    const name = result.name.padEnd(30);
    const duration = `${result.duration}ms`.padStart(8);
    console.log(`║ ${status} │ ${name} │ ${duration}       ║`);
  }

  console.log("╠════════════════════════════════════════════════════════════╣");
  console.log(`║ Total: ${passed} passed, ${failed} failed                              ║`);
  console.log(`║ Duration: ${suiteDuration}ms                                        ║`);
  console.log("╚════════════════════════════════════════════════════════════╝");

  if (failed > 0) {
    console.log("\nFailed tests:");
    for (const result of results.filter((r) => !r.passed)) {
      console.log(`  - ${result.name}`);
    }
    process.exit(1);
  }

  console.log("\nAll tests passed!");
}

main().catch((err) => {
  console.error("Test runner error:", err);
  process.exit(1);
});
