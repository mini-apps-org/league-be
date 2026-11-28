#!/usr/bin/env node
/**
 * Claude Code PreToolUse hook: Claude may NOT run `git commit` or `git push`.
 * Every other git command (stash, branch, checkout, switch, reset, apply, ...)
 * is left to the normal permission flow.
 *
 * Install at:  <project>/.claude/hooks/git-guard.js
 * Wired up in: <project>/.claude/settings.json
 *
 * Catches: git commit (incl. --amend), git push (incl. -f, --force,
 *   --force-with-lease, +refspec), also when written as `git -C dir commit`,
 *   `git -c k=v push`, `cd x && git commit`, `$(git commit ...)`,
 *   `bash -c "git push"`, `VAR=x git commit`, `/usr/bin/git push`.
 * Not blocked: `git stash push`, `git commit-tree`, `git log --grep=push`, ...
 *   (only the git SUBCOMMAND is checked, not words in its arguments).
 *
 * Plain Node, no imports and no packages.
 */

// "deny" = always blocked (Claude is told why).
// "ask"  = you get a permission prompt and can approve each one.
const MODE = "deny";

const BLOCKED = {
  commit: "'git commit' creates a commit",
  push: "'git push' sends commits to the remote",
};

// git global options that take a separate value: `git -C <dir> commit`.
const OPTS_WITH_VALUE = new Set([
  "-C", "-c", "--git-dir", "--work-tree", "--namespace", "--exec-path",
  "--super-prefix", "--config-env",
]);

/** Split a shell string into words, keeping quoted parts together. */
function words(segment) {
  const out = [];
  const re = /"((?:[^"\\]|\\.)*)"|'([^']*)'|(\S+)/g;
  let m;
  while ((m = re.exec(segment))) out.push(m[1] ?? m[2] ?? m[3]);
  return out;
}

/** Return the subcommand + args of every git call in a command line. */
function gitCalls(command) {
  const calls = [];
  // A git command can start at line start, after a separator, inside $( or
  // backticks or quotes, after VAR=x prefixes, or as a full path.
  const start = /(?:^|[\s;&|(`'"])(?:[\w./-]*\/)?git(?=\s|$)/g;
  let m;
  while ((m = start.exec(command))) {
    const rest = command.slice(m.index + m[0].length);
    const segment = rest.split(/[;&|\n`)]|\$\(/)[0]; // ends at next separator
    // Strip stray quotes so `bash -c "git push"` still reads as `push`.
    const w = words(segment).map((s) => s.replace(/^["']+|["']+$/g, ""));
    let i = 0;
    while (i < w.length && w[i].startsWith("-")) {
      const opt = w[i].split("=")[0];
      i += OPTS_WITH_VALUE.has(opt) && !w[i].includes("=") ? 2 : 1;
    }
    if (i < w.length) calls.push({ sub: w[i], args: w.slice(i + 1) });
  }
  return calls;
}

function check(command) {
  for (const { sub, args } of gitCalls(command)) {
    if (sub === "commit" && args.includes("--amend")) {
      return "'git commit --amend' rewrites a commit";
    }
    if (sub === "push" && args.some((a) =>
      /^(-f|--force|--force-with-lease|--force-if-includes)(=.*)?$/.test(a) || /^\+/.test(a))) {
      return "'git push --force' rewrites remote history";
    }
    if (BLOCKED[sub]) return BLOCKED[sub];
  }
  return null;
}

let raw = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (c) => (raw += c));
process.stdin.on("end", () => {
  let data;
  try {
    data = JSON.parse(raw);
  } catch {
    process.exit(0);
  }
  if (data.tool_name !== "Bash") process.exit(0);

  const reason = check((data.tool_input || {}).command || "");
  if (reason) {
    process.stdout.write(
      JSON.stringify({
        hookSpecificOutput: {
          hookEventName: "PreToolUse",
          permissionDecision: MODE,
          permissionDecisionReason:
            `Git policy: ${reason}. Claude must not commit or push. ` +
            "Leave the changes staged or stashed and tell the user what to commit or push.",
        },
      }) + "\n"
    );
  }
  process.exit(0);
});