import { test, before, after } from "node:test";
import assert from "node:assert/strict";
import os from "node:os";
import fs from "node:fs";
import path from "node:path";

process.env.DATA_DIR = fs.mkdtempSync(path.join(os.tmpdir(), "spam-test-"));
const { server } = await import("./server.js");
let base;
before(() => new Promise((r) => server.listen(0, () => ((base = `http://127.0.0.1:${server.address().port}`), r()))));
after(() => server.close());

async function call(method, url, body, cookie) {
  const res = await fetch(base + url, {
    method,
    headers: { ...(body ? { "Content-Type": "application/json" } : {}), ...(cookie ? { Cookie: cookie } : {}) },
    body: body ? JSON.stringify(body) : undefined,
  });
  const sc = res.headers.get("set-cookie");
  return { status: res.status, json: await res.json().catch(() => null), cookie: sc?.split(";")[0] };
}

test("full thread flow", async () => {
  const a = await call("POST", "/api/register", { username: "alice", password: "password123" });
  assert.equal(a.status, 201);
  const b = await call("POST", "/api/register", { username: "bob", password: "password123" });
  assert.equal((await call("POST", "/api/register", { username: "ALICE", password: "password123" })).status, 409);
  assert.equal((await call("POST", "/api/threads", { title: "x" })).status, 401);

  const t = await call("POST", "/api/threads", { title: "Hello", body: "first!" }, a.cookie);
  assert.equal(t.status, 201);
  const id = t.json.id;
  const r1 = await call("POST", `/api/threads/${id}/replies`, { body: "top" }, b.cookie);
  const r2 = await call("POST", `/api/threads/${id}/replies`, { body: "nested", parent_id: r1.json.id }, a.cookie);
  assert.equal(r2.status, 201);
  assert.equal((await call("POST", `/api/threads/${id}/replies`, { body: "x", parent_id: 9999 }, a.cookie)).status, 400);

  const like = await call("POST", `/api/threads/${id}/like`, null, b.cookie);
  assert.deepEqual(like.json, { liked: true, likes: 1 });
  assert.deepEqual((await call("POST", `/api/threads/${id}/like`, null, b.cookie)).json, { liked: false, likes: 0 });

  const view = await call("GET", `/api/threads/${id}`, null, a.cookie);
  assert.equal(view.json.replies.length, 2);
  assert.equal(view.json.replies[1].parent_id, r1.json.id);
  assert.equal(view.json.thread.mine, true);

  const feed = await call("GET", "/api/threads?sort=top");
  assert.equal(feed.json.threads[0].reply_count, 2);

  // only the owner can delete
  assert.equal((await call("DELETE", `/api/replies/${r1.json.id}`, null, a.cookie)).status, 404);
  assert.equal((await call("DELETE", `/api/replies/${r1.json.id}`, null, b.cookie)).status, 200);
  const after = await call("GET", `/api/threads/${id}`);
  assert.equal(after.json.replies[0].deleted, true);
  assert.equal(after.json.replies[0].author, null);
});

test("auth + validation", async () => {
  assert.equal((await call("POST", "/api/register", { username: "a b", password: "password123" })).status, 400);
  assert.equal((await call("POST", "/api/register", { username: "shorty", password: "short" })).status, 400);
  const bad = await call("POST", "/api/login", { username: "alice", password: "nope-nope" });
  assert.equal(bad.status, 401);
  const ok = await call("POST", "/api/login", { username: "alice", password: "password123" });
  assert.equal(ok.status, 200);
  assert.equal((await call("GET", "/api/me", null, ok.cookie)).json.user.username, "alice");
  await call("POST", "/api/logout", {}, ok.cookie);
  assert.equal((await call("GET", "/api/me", null, ok.cookie)).json.user, null);
  const form = await fetch(base + "/api/login", { method: "POST", headers: { "Content-Type": "text/plain" }, body: "{}" });
  assert.equal(form.status, 415);
});

test("static + SPA fallback + no path traversal", async () => {
  assert.equal((await fetch(base + "/")).status, 200);
  const spa = await fetch(base + "/t/1");
  assert.match(await spa.text(), /<title>spam/);
  assert.equal((await fetch(base + "/app.js")).status, 200);
  assert.equal((await fetch(base + "/missing.png")).status, 404);
  const trav = await fetch(base + "/..%2fserver.js");
  assert.notEqual(trav.headers.get("content-type"), "text/javascript; charset=utf-8");
  assert.equal((await fetch(base + "/api/health")).status, 200);
});
