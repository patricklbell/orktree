import { tool } from "@opencode-ai/plugin"
import { createHash } from "crypto"
import { existsSync, readdirSync, readFileSync, rmSync } from "fs"
import { homedir } from "os"
import { join } from "path"

function wardenStateDir(): string {
  return process.env.WARDEN_STATE_DIR ?? join(homedir(), ".orktree-warden")
}

function repoKey(root: string): string {
  return createHash("sha1").update(root).digest("hex").slice(0, 16)
}

function runsDir(root: string): string {
  return join(wardenStateDir(), "runs", repoKey(root))
}

export default tool({
  description:
    "Remove stale orchestrator run records and their orktree branches. " +
    "Runs are considered stale once their TTL has elapsed. " +
    "Use reap_finished to also remove runs whose TTL has not yet expired.",
  args: {
    repo_root: tool.schema
      .string()
      .optional()
      .describe("Absolute path to the git repository root. Defaults to the current worktree root."),
    reap_finished: tool.schema
      .boolean()
      .optional()
      .describe("Also remove runs that have not yet reached TTL expiry (default: false)"),
  },
  async execute(args, context) {
    const repoRoot = args.repo_root ?? context.worktree
    const rd = runsDir(repoRoot)

    if (!existsSync(rd)) return JSON.stringify({ cleaned: 0, kept: 0 })

    const nowSec = Math.floor(Date.now() / 1000)
    let cleaned = 0
    let kept = 0

    for (const f of readdirSync(rd).filter((n) => n.endsWith(".json"))) {
      const runFile = join(rd, f)
      let record: Record<string, unknown>
      try {
        record = JSON.parse(readFileSync(runFile, "utf8"))
      } catch {
        rmSync(runFile, { force: true })
        cleaned++
        continue
      }

      const createdAt = (record.created_at as number) ?? 0
      const ttl = (record.ttl_seconds as number) ?? 14400
      const expired = nowSec - createdAt >= ttl

      if (!expired && !args.reap_finished) {
        kept++
        continue
      }

      // Best-effort orktree removal; prefer workspace path (more robust), fall back to branch.
      const worktree = (record.workspace_path as string) || (record.branch as string)
      if (worktree) {
        await Bun.$`orktree rm ${worktree} --force`.cwd(repoRoot).nothrow().quiet()
      }
      rmSync(runFile, { force: true })
      cleaned++
    }

    return JSON.stringify({ cleaned, kept })
  },
})
