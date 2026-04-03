import { tool } from "@opencode-ai/plugin"
import { createHash } from "crypto"
import { existsSync, readdirSync, readFileSync } from "fs"
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
  description: "List recent orchestrator runs provisioned by the warden for this repository.",
  args: {
    repo_root: tool.schema
      .string()
      .optional()
      .describe("Absolute path to the git repository root. Defaults to the current worktree root."),
  },
  async execute(args, context) {
    const repoRoot = args.repo_root ?? context.worktree
    const rd = runsDir(repoRoot)

    if (!existsSync(rd)) return JSON.stringify({ runs: [] })

    const runs = readdirSync(rd)
      .filter((f) => f.endsWith(".json"))
      .map((f) => {
        try {
          return JSON.parse(readFileSync(join(rd, f), "utf8"))
        } catch {
          return null
        }
      })
      .filter(Boolean)

    return JSON.stringify({ runs })
  },
})
