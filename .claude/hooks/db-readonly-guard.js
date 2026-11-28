#!/usr/bin/env node
/**
 * Claude Code PreToolUse hook: allow Claude only READ access to databases.
 *
 * Install at:  <project>/.claude/hooks/db-readonly-guard.js
 * Wired up in: <project>/.claude/settings.json
 *
 * What it checks
 *   1. Bash commands that call a database client (psql, mysql, sqlite3, mongosh,
 *      redis-cli, sqlcmd, clickhouse-client, ...) or a migration tool.
 *   2. Calls to MCP database tools (any MCP server whose name looks like a DB).
 *
 * Any write / schema / permission statement is denied and the reason is sent
 * back to Claude, so it can retry with a read-only query.
 *
 * Plain Node, no imports and no packages, so it runs whether the project's
 * package.json uses "type": "module" or CommonJS.
 */

// Build a case-insensitive regex from pieces (keeps long patterns readable).
const rx = (parts, flags = "i") => new RegExp(parts.join(""), flags);

// ------------------------------------------------------------------ config --
// CLI clients that talk to a database.
const DB_CLIENTS = rx([
  String.raw`(?:^|[\s;&|(\x60$/])(`,
  String.raw`psql|pg_restore|pg_dump|mysql|mariadb|mysqlimport|sqlite3|duckdb|`,
  String.raw`mongo|mongosh|mongoimport|mongorestore|redis-cli|sqlcmd|`,
  String.raw`clickhouse-client|clickhouse|cockroach|cqlsh|snowsql|bq|influx|`,
  String.raw`supabase|turso|planetscale|pscale|neonctl`,
  String.raw`)(?=\s|$)`,
]);

// Commands that change a database even without SQL text in the command.
const WRITE_TOOLS = rx([
  String.raw`(?:^|[\s;&|(])(`,
  String.raw`pg_restore|mysqlimport|mongoimport|mongorestore|`,
  String.raw`prisma\s+(?:migrate|db\s+push|db\s+seed)|`,
  String.raw`(?:rails|rake)\s+db:(?!schema:dump|version|migrate:status)\S*|`,
  String.raw`manage\.py\s+(?:migrate|flush|loaddata|sqlflush)|`,
  String.raw`alembic\s+(?:upgrade|downgrade|stamp)|`,
  String.raw`(?:knex|sequelize|typeorm|drizzle-kit|flyway|liquibase|goose|`,
  String.raw`migrate|dbmate|atlas)\s+\S*(?:migrat|up|down|push|apply|reset|`,
  String.raw`seed|drop|update|run)\S*|`,
  String.raw`supabase\s+db\s+(?:push|reset)|`,
  String.raw`redis-cli\s+(?:.*\s)?(?:flushall|flushdb)`,
  String.raw`)`,
]);

// SQL statements that write data, change schema, or change permissions.
const SQL_WRITE = rx([
  String.raw`\b(`,
  String.raw`insert|update|delete|merge|upsert|replace\s+into|truncate|`,
  String.raw`create|alter|drop|rename|comment\s+on|`,
  String.raw`grant|revoke|`,
  String.raw`copy\s+\S+\s+from|load\s+data|import|`,
  String.raw`call|exec|execute|`,
  String.raw`vacuum|reindex|cluster|refresh\s+materialized|`,
  String.raw`lock\s+table|attach|detach|`,
  String.raw`set\s+(?:global|persist)|`,
  String.raw`select\b[^;]*\binto\b|for\s+update`,
  String.raw`)\b`,
]);

// Write statements that start or end on a non-word character (`\i`, `$$`, `=`),
// where the `\b` around SQL_WRITE can never match.
const SQL_WRITE_SYMBOLS = rx([
  String.raw`(`,
  String.raw`\bdo\s+\$|\bpragma\s+\w+\s*=|`,
  String.raw`\\copy\s+\S+\s+from|\\ir?\b`,
  String.raw`)`,
]);

// MongoDB shell / driver write methods.
const MONGO_WRITE = rx([
  String.raw`\.(insert\w*|update\w*|delete\w*|remove|replaceOne|drop\w*|`,
  String.raw`findOneAnd(?:Update|Replace|Delete)|findAndModify|bulkWrite|`,
  String.raw`create\w*|rename\w*|aggregate\([^)]*\$(?:out|merge))\b`,
]);

// Redis write commands (when the command comes after redis-cli).
const REDIS_WRITE = rx([
  String.raw`\bredis-cli\b.*\s(`,
  String.raw`set|setex|setnx|mset|del|unlink|expire|persist|rename|incr\w*|decr\w*|`,
  String.raw`append|lpush|rpush|lpop|rpop|lset|lrem|sadd|srem|smove|zadd|zrem\w*|`,
  String.raw`hset|hmset|hdel|hincr\w*|xadd|xdel|xtrim|flushall|flushdb|config\s+set|`,
  String.raw`eval|evalsha|script|restore|move|copy`,
  String.raw`)\b`,
]);

// Feeding a .sql / .js file into a client can't be inspected safely.
// `-f` is case-sensitive: psql's `-F` is the field separator, not a file.
const SCRIPT_FILE_FLAG = /\s-f\s/;
const SCRIPT_FILE = /(?:\s--file[=\s]|<\s*\S+\.(?:sql|js)\b|\s\S+\.(?:sql|js)\b)/i;

// MCP servers treated as databases (matched against the server part of
// mcp__<server>__<tool>). Whole name parts only, so `feedback` is not `db`.
// Add your own server names here.
const MCP_DB_SERVER = rx([
  String.raw`(?:^|[_-])(?:`,
  String.raw`postgres(?:ql)?|pg|mysql|maria(?:db)?|sqlite3?|mongo(?:db)?|redis|`,
  String.raw`supabase|neon|planetscale|bigquery|snowflake|clickhouse|duckdb|`,
  String.raw`databases?|db|sql`,
  String.raw`)(?:[_-]|$)`,
]);

// MCP tools whose NAME says they write, whatever their arguments.
// (query/execute_sql tools are allowed through and their SQL is inspected.)
const MCP_WRITE_TOOL = rx([
  String.raw`(?:^|_)(write|insert|update|delete|create|drop|alter|`,
  String.raw`migrate|apply|truncate|upsert|replace|put|set|remove)(?:_|$)`,
]);
// ----------------------------------------------------------------------------

/**
 * Remove comments and quoted strings so `SELECT 'drop'` is not blocked.
 * Line comments (--) are NOT stripped for shell commands, because CLI flags
 * like --eval / --command look the same and stripping them would hide SQL.
 */
function stripSqlNoise(text, lineComments = true) {
  text = text.replace(/\/\*[\s\S]*?\*\//g, " ");
  if (lineComments) text = text.replace(/--[^\n]*/g, " ");
  text = text.replace(/'(?:[^'\\]|\\.|'')*'/g, "''");
  return text;
}

class Denied extends Error {}

function deny(reason) {
  throw new Denied(
    `Database is READ-ONLY for Claude: ${reason}. ` +
      "Only SELECT/SHOW/DESCRIBE/EXPLAIN-style reads are allowed. " +
      "Ask the user to run write operations themselves."
  );
}

function checkText(text, where, lineComments = true) {
  const cleaned = stripSqlNoise(text, lineComments);
  let m = cleaned.match(SQL_WRITE);
  if (m) deny(`'${m[1].toUpperCase()}' statement in ${where}`);
  m = cleaned.match(SQL_WRITE_SYMBOLS);
  if (m) deny(`'${m[1].trim().toUpperCase()}' statement in ${where}`);
  m = cleaned.match(MONGO_WRITE);
  if (m) deny(`MongoDB write method '.${m[1]}' in ${where}`);
}

function checkBash(command) {
  let m = command.match(WRITE_TOOLS);
  if (m) deny(`'${m[1].trim()}' modifies the database`);

  const client = command.match(DB_CLIENTS);
  if (!client) return; // not a DB command; normal permissions

  m = command.match(REDIS_WRITE);
  if (m) deny(`Redis write command '${m[1].toUpperCase()}'`);

  // Only from the client onward, so `test -f .env && psql ...` is not a script file.
  const fromClient = command.slice(client.index);
  if (SCRIPT_FILE_FLAG.test(fromClient) || SCRIPT_FILE.test(fromClient)) {
    deny(
      "running a SQL/script file through a DB client can't be verified as " +
        "read-only; pass the query inline with -c/-e"
    );
  }

  checkText(command, "Bash command", false);
}

function* collectStrings(value) {
  if (typeof value === "string") yield value;
  else if (Array.isArray(value)) for (const v of value) yield* collectStrings(v);
  else if (value && typeof value === "object")
    for (const v of Object.values(value)) yield* collectStrings(v);
}

function checkMcp(toolName, toolInput) {
  const parts = toolName.split("__");
  if (parts.length < 3) return;
  const server = parts[1];
  const tool = parts.slice(2).join("__");
  if (!MCP_DB_SERVER.test(server)) return;
  if (MCP_WRITE_TOOL.test(tool)) {
    deny(`MCP tool '${tool}' on server '${server}' performs writes`);
  }
  for (const s of collectStrings(toolInput)) {
    checkText(s, `MCP tool '${toolName}'`);
  }
}

function evaluate(data) {
  const toolName = data.tool_name || "";
  const toolInput = data.tool_input || {};
  if (toolName === "Bash") checkBash(toolInput.command || "");
  else if (toolName.startsWith("mcp__")) checkMcp(toolName, toolInput);
}

let raw = "";
process.stdin.setEncoding("utf8");
process.stdin.on("data", (chunk) => (raw += chunk));
process.stdin.on("end", () => {
  let data;
  try {
    data = JSON.parse(raw);
  } catch {
    process.exit(0); // unreadable input: no decision
  }

  try {
    evaluate(data);
  } catch (err) {
    if (!(err instanceof Denied)) throw err;
    process.stdout.write(
      JSON.stringify({
        hookSpecificOutput: {
          hookEventName: "PreToolUse",
          permissionDecision: "deny",
          permissionDecisionReason: err.message,
        },
      }) + "\n"
    );
  }
  process.exit(0); // no output = no decision -> normal permission flow
});