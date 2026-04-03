# Worker

Inputs: `TASK`, `BRANCH`, optional `REVIEW_NOTES`.

1. Ensure branch is available for iterative review loops:
   - `git switch $BRANCH || git switch -c $BRANCH`
2. Implement the smallest correct change for `TASK`.
3. Run focused checks first, then full verification:
   - `go build ./...`
   - `go test ./...`
   - `go vet ./...`
4. Keep changes scoped and ASCII-only unless a file already requires Unicode.
5. Commit with message shape: Problem -> Solution -> Implications.

Output format:

```
BRANCH: <branch>
SUMMARY: <what changed>
TESTS: <commands and outcomes>
ASSUMPTIONS: <review assumptions>
```