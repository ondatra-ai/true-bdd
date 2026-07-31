/**
 * AI banner reporter (plan §4.4): "skipped ≠ green". Prints a LOUD
 * `AI: N passed, M skipped — <reason>` banner at the end of every run
 * and writes a machine-readable summary to
 * `test-results/ai-report.json`, so a fully-skipped AI project can
 * never be mistaken for a passing one.
 */

import fs from "node:fs";
import path from "node:path";

import type {
  FullConfig,
  FullResult,
  Reporter,
  TestCase,
  TestResult,
} from "@playwright/test/reporter";

interface AiTestRecord {
  title: string;
  file: string;
  status: TestResult["status"];
  durationMs: number;
  skipReason?: string;
}

interface AiReport {
  aiProjectConfigured: boolean;
  passed: number;
  failed: number;
  skipped: number;
  reason: string;
  gate?: unknown;
  tests: AiTestRecord[];
  overallStatus: FullResult["status"];
  finishedAt: string;
}

const BANNER_WIDTH = 72;

export default class AiBannerReporter implements Reporter {
  private records: AiTestRecord[] = [];
  private aiProjectConfigured = false;
  private rootDir = process.cwd();
  private harnessDir = process.cwd();

  onBegin(config: FullConfig): void {
    // config.rootDir is the resolved testDir; the report belongs in
    // the harness app dir's gitignored test-results/.
    this.rootDir = config.rootDir;
    this.harnessDir =
      config.configFile !== undefined ? path.dirname(config.configFile) : process.cwd();
    this.aiProjectConfigured = config.projects.some((project) => project.name === "ai");
  }

  onTestEnd(test: TestCase, result: TestResult): void {
    if (test.parent.project()?.name !== "ai") {
      return;
    }

    const skipAnnotation = test.annotations.find((a) => a.type === "skip");
    this.records.push({
      title: test.title,
      file: path.relative(this.rootDir, test.location.file),
      status: result.status,
      durationMs: result.duration,
      skipReason: skipAnnotation?.description ?? undefined,
    });
  }

  onEnd(result: FullResult): void {
    if (!this.aiProjectConfigured) {
      return;
    }

    const passed = this.records.filter((r) => r.status === "passed").length;
    const skipped = this.records.filter(
      (r) => r.status === "skipped" || r.status === "interrupted",
    ).length;
    const failed = this.records.filter(
      (r) => r.status === "failed" || r.status === "timedOut",
    ).length;

    const reason = this.resolveReason(skipped);

    const summary =
      failed > 0
        ? `AI: ${passed} passed, ${skipped} skipped, ${failed} FAILED — ${reason}`
        : `AI: ${passed} passed, ${skipped} skipped — ${reason}`;

    const bar = "=".repeat(BANNER_WIDTH);
    console.log(`\n${bar}\n${summary}\n${bar}\n`);

    this.writeReport(result, { passed, failed, skipped, reason });
  }

  printsToStdio(): boolean {
    return false; // keep the list reporter as the primary stdio owner
  }

  /**
   * Reason precedence: gate file (authoritative — why the whole
   * project skipped) → first per-test skip reason → generic text.
   */
  private resolveReason(skipped: number): string {
    const gate = this.readGate();
    if (gate !== undefined) {
      const doc = gate as { ok?: boolean; reason?: string; model?: string };
      if (doc.ok === false) {
        return doc.reason ?? "claude unavailable";
      }

      if (doc.ok === true && skipped === 0) {
        return `claude available (model ${doc.model ?? "unknown"})`;
      }
    }

    const firstSkip = this.records.find((r) => r.skipReason !== undefined);
    if (firstSkip?.skipReason !== undefined) {
      return firstSkip.skipReason;
    }

    if (this.records.length === 0) {
      return "no AI tests were scheduled in this run";
    }

    return skipped > 0 ? "tests skipped (no recorded reason)" : "all AI tests executed";
  }

  private readGate(): unknown {
    const suiteRoot = process.env.TRUE_BDD_E2E_SUITE_ROOT;
    if (suiteRoot === undefined) {
      return undefined;
    }

    try {
      return JSON.parse(fs.readFileSync(path.join(suiteRoot, "ai-gate.json"), "utf8"));
    } catch {
      return undefined;
    }
  }

  private writeReport(
    result: FullResult,
    counts: { passed: number; failed: number; skipped: number; reason: string },
  ): void {
    const report: AiReport = {
      aiProjectConfigured: this.aiProjectConfigured,
      passed: counts.passed,
      failed: counts.failed,
      skipped: counts.skipped,
      reason: counts.reason,
      gate: this.readGate(),
      tests: this.records,
      overallStatus: result.status,
      finishedAt: new Date().toISOString(),
    };

    const outDir = path.join(this.harnessDir, "test-results");
    fs.mkdirSync(outDir, { recursive: true });
    fs.writeFileSync(
      path.join(outDir, "ai-report.json"),
      `${JSON.stringify(report, null, 2)}\n`,
    );
  }
}
