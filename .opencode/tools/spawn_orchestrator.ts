import { tool } from "@opencode-ai/plugin"
import { createHash, randomBytes } from "crypto"
import { mkdirSync, renameSync, writeFileSync } from "fs"
import { homedir } from "os"
import { join } from "path"

// Provision a new orktree worktree at the given path and return it.
async function orktreeAdd(repoRoot: string, workspacePath: string, branch: string): Promise<string> {
  // -- separates orktree flags from git-worktree-add flags; -b sets the branch name
  // explicitly so it matches the warden/<rid> convention instead of defaulting to
  // the basename of workspacePath.
  await Bun.$`orktree add ${workspacePath} -- -b ${branch}`.cwd(repoRoot)
  return workspacePath
}

function wardenStateDir(): string {
  return process.env.WARDEN_STATE_DIR ?? join(homedir(), ".orktree-warden")
}

function repoKey(root: string): string {
  return createHash("sha1").update(root).digest("hex").slice(0, 16)
}

function runsDir(root: string): string {
  return join(wardenStateDir(), "runs", repoKey(root))
}

function runId(label?: string): string {
  const raw = randomBytes(6).toString("hex").slice(0, 12)
  if (!label) return raw
  const safeLabel = label.replace(/[^a-zA-Z0-9-]/g, "-").slice(0, 40)
  return `${raw}-${safeLabel}`
}

export default tool({
  description:
    "Provision an isolated orktree branch for one orchestrator task. " +
    "Returns the branch name and workspace path. " +
    "Call this before spawning each orchestrator via the Task tool so every task " +
    "gets its own copy-on-write workspace and branch with no file-level contention.",
  args: {
    label: tool.schema
      .string()
      .optional()
      .describe("Short human-readable label included in the branch name (e.g. 'fix-parser')"),
    repo_root: tool.schema
      .string()
      .optional()
      .describe("Absolute path to the git repository root. Defaults to the current worktree root."),
    ttl_seconds: tool.schema
      .number()
      .optional()
      .describe("Seconds before the run is considered stale by reap_stale_runs (default: 14400)"),
  },
  async execute(args, context) {
    const repoRoot = args.repo_root ?? context.worktree
    const ttl = args.ttl_seconds ?? 14400

    const rid = runId(args.label)
    const branch = `warden/${rid}`

    const workspace = `${repoRoot}.orktree/warden/${rid}`
    await orktreeAdd(repoRoot, workspace, branch)

    const rd = runsDir(repoRoot)
    mkdirSync(rd, { recursive: true })

    const runFile = join(rd, `${rid}.json`)
    const runFileTmp = `${runFile}.tmp.${process.pid}`
    const record = JSON.stringify(
      {
        run_id: rid,
        branch,
        repo_root: repoRoot,
        workspace_path: workspace,
        created_at: Math.floor(Date.now() / 1000),
        ttl_seconds: ttl,
      },
      null,
      2,
    )
    writeFileSync(runFileTmp, record, { mode: 0o600 })
    renameSync(runFileTmp, runFile)

    return JSON.stringify({ run_id: rid, branch, workspace_path: workspace })
  },
})
