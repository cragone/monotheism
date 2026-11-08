// spam — a small threads-based social site. One process: static UI + JSON API + SQLite.
import http from "node:http";
import fs from "node:fs";
import path from "node:path";
import crypto from "node:crypto";
import { fileURLToPath } from "node:url";
import { DatabaseSync } from "node:sqlite";

const __dirname = path.dirname(fileURLToPath(import.meta.url));
const PUBLIC_DIR = path.join(__dirname, "public");
const DATA_DIR = process.env.DATA_DIR || path.join(__dirname, "data");
const PORT = Number(process.env.PORT || 8080);
const PROD = process.env.NODE_ENV === "production";

const LIMITS = { title: 200, threadBody: 10000, reply: 5000, username: 24 };
const PAGE_SIZE = 20;
const SESSION_DAYS = 30;

fs.mkdirSync(DATA_DIR, { recursive: true });
export const db = new DatabaseSync(path.join(DATA_DIR, "spam.db"));
db.exec(`
  PRAGMA journal_mode = WAL;
  PRAGMA foreign_keys = ON;
  CREATE TABLE IF NOT EXISTS users (
    id INTEGER PRIMARY KEY,
    username TEXT NOT NULL UNIQUE COLLATE NOCASE,
    pass_hash TEXT NOT NULL,
    created_at INTEGER NOT NULL
  );
  CREATE TABLE IF NOT EXISTS sessions (
    token_hash TEXT PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id) ON DELETE CASCADE,
    expires_at INTEGER NOT NULL
  );
  CREATE TABLE IF NOT EXISTS threads (
    id INTEGER PRIMARY KEY,
    user_id INTEGER NOT NULL REFERENCES users(id),
    title TEXT NOT NULL,
    body TEXT NOT NULL DEFAULT '',
    created_at INTEGER NOT NULL,
    last_activity INTEGER NOT NULL,
    deleted INTEGER NOT NULL DEFAULT 0
  );
  CREATE TABLE IF NOT EXISTS replies (
    id INTEGER PRIMARY KEY,
    thread_id INTEGER NOT NULL REFERENCES threads(id),
    parent_id INTEGER REFERENCES replies(id),
    user_id INTEGER NOT NULL REFERENCES users(id),
    body TEXT NOT NULL,
    created_at INTEGER NOT NULL,
    deleted INTEGER NOT NULL DEFAULT 0
  );
  CREATE TABLE IF NOT EXISTS likes (
    user_id INTEGER NOT NULL REFERENCES users(id),
    kind TEXT NOT NULL CHECK (kind IN ('thread','reply')),
    target_id INTEGER NOT NULL,
    PRIMARY KEY (user_id, kind, target_id)
  );
  CREATE INDEX IF NOT EXISTS idx_threads_activity ON threads(last_activity DESC);
  CREATE INDEX IF NOT EXISTS idx_replies_thread ON replies(thread_id);
  CREATE INDEX IF NOT EXISTS idx_likes_target ON likes(kind, target_id);
`);

const now = () => Date.now();

// ---------- helpers ----------
class HttpError extends Error {
  constructor(status, message) {
    super(message);
    this.status = status;
  }
}
const bad = (msg) => new HttpError(400, msg);

function hashPassword(pw) {
  const salt = crypto.randomBytes(16);
  const key = crypto.scryptSync(pw, salt, 64);
  return `${salt.toString("hex")}:${key.toString("hex")}`;
}
function verifyPassword(pw, stored) {
  const [saltHex, keyHex] = stored.split(":");
  const key = crypto.scryptSync(pw, Buffer.from(saltHex, "hex"), 64);
  const expected = Buffer.from(keyHex, "hex");
  return key.length === expected.length && crypto.timingSafeEqual(key, expected);
}
const sha256 = (s) => crypto.createHash("sha256").update(s).digest("hex");

function parseCookies(header = "") {
  const out = {};
  for (const part of header.split(";")) {
    const i = part.indexOf("=");
    if (i > 0) out[part.slice(0, i).trim()] = decodeURIComponent(part.slice(i + 1).trim());
  }
  return out;
}

function clientIp(req) {
  return (
    req.headers["cf-connecting-ip"] ||
    (req.headers["x-forwarded-for"] || "").split(",")[0].trim() ||
    req.socket.remoteAddress ||
    "unknown"
  );
}

// Tiny in-memory fixed-window limiter. Traefik rate-limits the edge; this protects the
// expensive/abusable actions (password hashing, posting) per IP.
const buckets = new Map();
function rateLimit(key, max, windowMs) {
  const t = now();
  const b = buckets.get(key);
  if (!b || b.reset < t) {
    buckets.set(key, { count: 1, reset: t + windowMs });
    return;
  }
  if (++b.count > max) throw new HttpError(429, "Too many requests — slow down a bit.");
}
setInterval(() => {
  const t = now();
  for (const [k, b] of buckets) if (b.reset < t) buckets.delete(k);
  db.prepare("DELETE FROM sessions WHERE expires_at < ?").run(t);
}, 60_000).unref();

function readJson(req) {
  return new Promise((resolve, reject) => {
    if (!(req.headers["content-type"] || "").startsWith("application/json")) {
      return reject(new HttpError(415, "Content-Type must be application/json"));
    }
    let size = 0;
    const chunks = [];
    req.on("data", (c) => {
      size += c.length;
      if (size > 64 * 1024) {
        reject(new HttpError(413, "Request too large"));
        req.destroy();
      } else chunks.push(c);
    });
    req.on("end", () => {
      try {
        const v = JSON.parse(Buffer.concat(chunks).toString("utf8") || "{}");
        if (v === null || typeof v !== "object" || Array.isArray(v)) throw 0;
        resolve(v);
      } catch {
        reject(bad("Invalid JSON"));
      }
    });
    req.on("error", reject);
  });
}

function send(res, status, body, headers = {}) {
  const data = JSON.stringify(body);
  res.writeHead(status, {
    "Content-Type": "application/json; charset=utf-8",
    "Cache-Control": "no-store",
    ...headers,
  });
  res.end(data);
}

function setSessionCookie(res, token, maxAgeSec) {
  const attrs = [`sid=${token}`, "Path=/", "HttpOnly", "SameSite=Lax", `Max-Age=${maxAgeSec}`];
  if (PROD) attrs.push("Secure");
  return attrs.join("; ");
}

function currentUser(req) {
  const sid = parseCookies(req.headers.cookie).sid;
  if (!sid) return null;
  return (
    db
      .prepare(
        `SELECT u.id, u.username FROM sessions s JOIN users u ON u.id = s.user_id
         WHERE s.token_hash = ? AND s.expires_at > ?`
      )
      .get(sha256(sid), now()) || null
  );
}
function requireUser(req) {
  const u = currentUser(req);
  if (!u) throw new HttpError(401, "Log in first.");
  return u;
}

function cleanText(v, name, max, { required = true } = {}) {
  if (typeof v !== "string") throw bad(`${name} must be text`);
  const s = v.replace(/\r\n/g, "\n").trim();
  if (required && !s) throw bad(`${name} can't be empty`);
  if (s.length > max) throw bad(`${name} is too long (max ${max} characters)`);
  return s;
}

function startSession(res, userId) {
  const token = crypto.randomBytes(32).toString("base64url");
  db.prepare("INSERT INTO sessions (token_hash, user_id, expires_at) VALUES (?, ?, ?)").run(
    sha256(token),
    userId,
    now() + SESSION_DAYS * 86400_000
  );
  return setSessionCookie(res, token, SESSION_DAYS * 86400);
}

// ---------- queries ----------
const threadSelect = `
  SELECT t.id, t.title, t.body, t.created_at, t.last_activity, t.deleted,
         u.username AS author,
         (SELECT COUNT(*) FROM likes l WHERE l.kind='thread' AND l.target_id=t.id) AS likes,
         (SELECT COUNT(*) FROM replies r WHERE r.thread_id=t.id AND r.deleted=0) AS reply_count,
         EXISTS(SELECT 1 FROM likes l WHERE l.kind='thread' AND l.target_id=t.id AND l.user_id=:me) AS liked
  FROM threads t JOIN users u ON u.id = t.user_id`;

function shapeThread(row, me, { full = false } = {}) {
  const gone = row.deleted === 1;
  return {
    id: row.id,
    title: gone ? "[deleted]" : row.title,
    body: gone || !full ? (gone ? "" : row.body.slice(0, 280)) : row.body,
    truncated: !full && !gone && row.body.length > 280,
    author: gone ? null : row.author,
    mine: !!me && !gone && row.author.toLowerCase() === me.username.toLowerCase(),
    created_at: row.created_at,
    last_activity: row.last_activity,
    likes: row.likes,
    reply_count: row.reply_count,
    liked: !!row.liked,
    deleted: gone,
  };
}

// ---------- routes ----------
const routes = [];
const route = (method, pattern, handler) =>
  routes.push({ method, re: new RegExp(`^${pattern.replace(/:(\w+)/g, "(?<$1>[^/]+)")}$`), handler });

route("GET", "/api/health", async () => ({ ok: true }));

route("GET", "/api/me", async (req) => ({ user: currentUser(req) }));

const USERNAME_RE = /^[A-Za-z0-9_]{3,24}$/;
route("POST", "/api/register", async (req, res) => {
  rateLimit(`reg:${clientIp(req)}`, 5, 3600_000);
  const { username, password } = await readJson(req);
  if (typeof username !== "string" || !USERNAME_RE.test(username))
    throw bad("Username must be 3–24 letters, numbers or underscores");
  if (typeof password !== "string" || password.length < 8 || password.length > 200)
    throw bad("Password must be at least 8 characters");
  let id;
  try {
    id = db
      .prepare("INSERT INTO users (username, pass_hash, created_at) VALUES (?, ?, ?)")
      .run(username, hashPassword(password), now()).lastInsertRowid;
  } catch (e) {
    if (String(e.message).includes("UNIQUE")) throw new HttpError(409, "That username is taken");
    throw e;
  }
  return { status: 201, headers: { "Set-Cookie": startSession(res, Number(id)) }, body: { user: { id: Number(id), username } } };
});

route("POST", "/api/login", async (req, res) => {
  const ip = clientIp(req);
  rateLimit(`login:${ip}`, 20, 600_000);
  const { username, password } = await readJson(req);
  if (typeof username !== "string" || typeof password !== "string") throw bad("Username and password required");
  rateLimit(`login-user:${username.toLowerCase()}`, 10, 600_000);
  const row = db.prepare("SELECT id, username, pass_hash FROM users WHERE username = ?").get(username);
  // Hash even for unknown users so response time doesn't reveal which usernames exist.
  const ok = row ? verifyPassword(password, row.pass_hash) : (hashPassword(password), false);
  if (!ok) throw new HttpError(401, "Wrong username or password");
  return { headers: { "Set-Cookie": startSession(res, row.id) }, body: { user: { id: row.id, username: row.username } } };
});

route("POST", "/api/logout", async (req) => {
  const sid = parseCookies(req.headers.cookie).sid;
  if (sid) db.prepare("DELETE FROM sessions WHERE token_hash = ?").run(sha256(sid));
  return { headers: { "Set-Cookie": setSessionCookie("", "", 0) }, body: { ok: true } };
});

route("GET", "/api/threads", async (req, _res, url) => {
  const me = currentUser(req);
  const sort = url.searchParams.get("sort") || "active";
  const page = Math.max(0, parseInt(url.searchParams.get("page") || "0", 10) || 0);
  const order = {
    new: "t.created_at DESC",
    active: "t.last_activity DESC",
    top: "likes DESC, reply_count DESC, t.created_at DESC",
  }[sort];
  if (!order) throw bad("sort must be new, active or top");
  const rows = db
    .prepare(`${threadSelect} WHERE t.deleted = 0 ORDER BY ${order} LIMIT :lim OFFSET :off`)
    .all({ me: me?.id ?? 0, lim: PAGE_SIZE + 1, off: page * PAGE_SIZE });
  return {
    threads: rows.slice(0, PAGE_SIZE).map((r) => shapeThread(r, me)),
    has_more: rows.length > PAGE_SIZE,
  };
});

route("POST", "/api/threads", async (req) => {
  const user = requireUser(req);
  rateLimit(`post:${user.id}`, 10, 600_000);
  const b = await readJson(req);
  const title = cleanText(b.title, "Title", LIMITS.title);
  const body = cleanText(b.body ?? "", "Body", LIMITS.threadBody, { required: false });
  const t = now();
  const id = db
    .prepare("INSERT INTO threads (user_id, title, body, created_at, last_activity) VALUES (?, ?, ?, ?, ?)")
    .run(user.id, title, body, t, t).lastInsertRowid;
  return { status: 201, body: { id: Number(id) } };
});

function loadThread(id, me) {
  const row = db.prepare(`${threadSelect} WHERE t.id = :id`).get({ id, me: me?.id ?? 0 });
  if (!row) throw new HttpError(404, "Thread not found");
  return row;
}

route("GET", "/api/threads/:id", async (req, _res, url, { id }) => {
  const me = currentUser(req);
  const row = loadThread(Number(id), me);
  const replies = db
    .prepare(
      `SELECT r.id, r.parent_id, r.body, r.created_at, r.deleted, u.username AS author,
              (SELECT COUNT(*) FROM likes l WHERE l.kind='reply' AND l.target_id=r.id) AS likes,
              EXISTS(SELECT 1 FROM likes l WHERE l.kind='reply' AND l.target_id=r.id AND l.user_id=:me) AS liked
       FROM replies r JOIN users u ON u.id = r.user_id WHERE r.thread_id = :id ORDER BY r.created_at ASC, r.id ASC`
    )
    .all({ id: Number(id), me: me?.id ?? 0 });
  return {
    thread: shapeThread(row, me, { full: true }),
    replies: replies.map((r) => ({
      id: r.id,
      parent_id: r.parent_id,
      body: r.deleted ? "" : r.body,
      author: r.deleted ? null : r.author,
      mine: !!me && !r.deleted && r.author.toLowerCase() === me.username.toLowerCase(),
      created_at: r.created_at,
      likes: r.likes,
      liked: !!r.liked,
      deleted: r.deleted === 1,
    })),
  };
});

route("POST", "/api/threads/:id/replies", async (req, _res, _url, { id }) => {
  const user = requireUser(req);
  rateLimit(`reply:${user.id}`, 30, 600_000);
  const b = await readJson(req);
  const body = cleanText(b.body, "Reply", LIMITS.reply);
  const threadId = Number(id);
  const thread = db.prepare("SELECT id, deleted FROM threads WHERE id = ?").get(threadId);
  if (!thread || thread.deleted) throw new HttpError(404, "Thread not found");
  let parentId = null;
  if (b.parent_id != null) {
    parentId = Number(b.parent_id);
    const p = db.prepare("SELECT thread_id FROM replies WHERE id = ?").get(parentId);
    if (!p || p.thread_id !== threadId) throw bad("Parent reply not found in this thread");
  }
  const t = now();
  const rid = db
    .prepare("INSERT INTO replies (thread_id, parent_id, user_id, body, created_at) VALUES (?, ?, ?, ?, ?)")
    .run(threadId, parentId, user.id, body, t).lastInsertRowid;
  db.prepare("UPDATE threads SET last_activity = ? WHERE id = ?").run(t, threadId);
  return { status: 201, body: { id: Number(rid) } };
});

function toggleLike(kind, table) {
  return async (req, _res, _url, { id }) => {
    const user = requireUser(req);
    rateLimit(`like:${user.id}`, 120, 600_000);
    const target = Number(id);
    if (!db.prepare(`SELECT 1 FROM ${table} WHERE id = ? AND deleted = 0`).get(target))
      throw new HttpError(404, "Not found");
    const del = db.prepare("DELETE FROM likes WHERE user_id=? AND kind=? AND target_id=?").run(user.id, kind, target);
    if (del.changes === 0) db.prepare("INSERT INTO likes (user_id, kind, target_id) VALUES (?, ?, ?)").run(user.id, kind, target);
    const likes = db.prepare("SELECT COUNT(*) AS c FROM likes WHERE kind=? AND target_id=?").get(kind, target).c;
    return { liked: del.changes === 0, likes };
  };
}
route("POST", "/api/threads/:id/like", toggleLike("thread", "threads"));
route("POST", "/api/replies/:id/like", toggleLike("reply", "replies"));

function softDelete(table) {
  return async (req, _res, _url, { id }) => {
    const user = requireUser(req);
    const r = db.prepare(`UPDATE ${table} SET deleted = 1 WHERE id = ? AND user_id = ?`).run(Number(id), user.id);
    if (r.changes === 0) throw new HttpError(404, "Not found, or not yours");
    return { ok: true };
  };
}
route("DELETE", "/api/threads/:id", softDelete("threads"));
route("DELETE", "/api/replies/:id", softDelete("replies"));

// ---------- static files ----------
const MIME = {
  ".html": "text/html; charset=utf-8",
  ".js": "text/javascript; charset=utf-8",
  ".css": "text/css; charset=utf-8",
  ".svg": "image/svg+xml",
  ".png": "image/png",
  ".ico": "image/x-icon",
};
const SEC_HEADERS = {
  "X-Content-Type-Options": "nosniff",
  "X-Frame-Options": "DENY",
  "Referrer-Policy": "strict-origin-when-cross-origin",
  "Content-Security-Policy":
    "default-src 'self'; img-src 'self' data:; style-src 'self'; script-src 'self'; frame-ancestors 'none'; base-uri 'none'; form-action 'self'",
};

function serveStatic(req, res, pathname) {
  let rel = decodeURIComponent(pathname);
  if (rel === "/") rel = "/index.html";
  const file = path.normalize(path.join(PUBLIC_DIR, rel));
  let target = file.startsWith(PUBLIC_DIR + path.sep) && fs.existsSync(file) && fs.statSync(file).isFile() ? file : null;
  // SPA fallback: unknown extension-less paths (e.g. /t/12) load the app shell.
  if (!target && !path.extname(rel)) target = path.join(PUBLIC_DIR, "index.html");
  if (!target) {
    res.writeHead(404, { "Content-Type": "text/plain" });
    return res.end("Not found");
  }
  const ext = path.extname(target);
  res.writeHead(200, {
    "Content-Type": MIME[ext] || "application/octet-stream",
    "Cache-Control": ext === ".html" ? "no-cache" : "public, max-age=300",
    ...SEC_HEADERS,
  });
  if (req.method === "HEAD") return res.end();
  fs.createReadStream(target).pipe(res);
}

// ---------- server ----------
export const server = http.createServer(async (req, res) => {
  try {
    const url = new URL(req.url, "http://x");
    if (!url.pathname.startsWith("/api/")) {
      if (req.method !== "GET" && req.method !== "HEAD") throw new HttpError(405, "Method not allowed");
      return serveStatic(req, res, url.pathname);
    }
    for (const r of routes) {
      if (r.method !== req.method) continue;
      const m = r.re.exec(url.pathname);
      if (!m) continue;
      const out = await r.handler(req, res, url, m.groups || {});
      if (out && "body" in out) return send(res, out.status || 200, out.body, out.headers);
      return send(res, 200, out);
    }
    throw new HttpError(404, "Not found");
  } catch (e) {
    if (!(e instanceof HttpError)) console.error(e);
    if (res.headersSent) return res.end();
    const status = e instanceof HttpError ? e.status : 500;
    send(res, status, { error: status === 500 ? "Internal error" : e.message });
  }
});

if (process.argv[1] === fileURLToPath(import.meta.url)) {
  server.listen(PORT, () => console.log(`spam listening on :${PORT}`));
  for (const sig of ["SIGTERM", "SIGINT"]) process.on(sig, () => server.close(() => process.exit(0)));
}
