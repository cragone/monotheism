// spam front-end: tiny vanilla SPA (history API). All user text is inserted via textContent.
const $app = document.getElementById("app");
const $nav = document.getElementById("nav");
const state = { user: null, sort: "active" };

// ---- tiny DOM helper ----
function h(tag, props = {}, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props)) {
    if (k.startsWith("on")) el.addEventListener(k.slice(2), v);
    else if (k === "class") el.className = v;
    else if (v === true) el.setAttribute(k, "");
    else if (v !== false && v != null) el.setAttribute(k, v);
  }
  for (const kid of kids.flat()) if (kid != null && kid !== false) el.append(kid);
  return el;
}

async function api(method, url, body) {
  const res = await fetch(url, {
    method,
    headers: body ? { "Content-Type": "application/json" } : {},
    body: body ? JSON.stringify(body) : undefined,
    credentials: "same-origin",
  });
  const data = await res.json().catch(() => ({}));
  if (!res.ok) throw new Error(data.error || `Request failed (${res.status})`);
  return data;
}

function ago(ts) {
  const s = Math.max(1, Math.round((Date.now() - ts) / 1000));
  const units = [[60, "s"], [60, "m"], [24, "h"], [30, "d"], [12, "mo"]];
  let n = s;
  for (const [div, label] of units) {
    if (n < div) return `${n}${label}`;
    n = Math.floor(n / div);
  }
  return `${n}y`;
}

function go(path) {
  history.pushState({}, "", path);
  render();
}
document.addEventListener("click", (e) => {
  const a = e.target.closest("a[data-link]");
  if (a && !e.metaKey && !e.ctrlKey && !e.shiftKey && e.button === 0) {
    e.preventDefault();
    go(a.getAttribute("href"));
  }
});
window.addEventListener("popstate", render);

// ---- nav ----
function renderNav() {
  $nav.replaceChildren(
    ...(state.user
      ? [
          h("span", { class: "meta" }, `@${state.user.username}`),
          h("a", { class: "btn primary", href: "/new", "data-link": true }, "New thread"),
          h("button", { onclick: async () => { await api("POST", "/api/logout", {}); state.user = null; renderNav(); render(); } }, "Log out"),
        ]
      : [h("a", { class: "btn", href: "/login", "data-link": true }, "Log in"), h("a", { class: "btn primary", href: "/register", "data-link": true }, "Sign up")])
  );
}

// ---- shared pieces ----
function likeButton(item, kind) {
  const btn = h("button", { class: "link like", "aria-pressed": String(item.liked), title: "Like" });
  const paint = () => {
    btn.textContent = `${item.liked ? "♥" : "♡"} ${item.likes}`;
    btn.setAttribute("aria-pressed", String(item.liked));
  };
  paint();
  btn.addEventListener("click", async () => {
    if (!state.user) return go("/login");
    try {
      const r = await api("POST", `/api/${kind}/${item.id}/like`);
      item.liked = r.liked;
      item.likes = r.likes;
      paint();
    } catch (e) { alert(e.message); }
  });
  return btn;
}

function composer({ placeholder, submitLabel, onSubmit, withTitle = false, onCancel }) {
  const title = withTitle ? h("input", { placeholder: "Title", maxlength: 200, required: true }) : null;
  const body = h("textarea", { placeholder, maxlength: withTitle ? 10000 : 5000, required: !withTitle });
  const err = h("div", { class: "err" });
  const submit = h("button", { class: "primary", type: "submit" }, submitLabel);
  const form = h("form", { class: "compose" }, title, body, err,
    h("div", { class: "row" }, onCancel && h("button", { type: "button", onclick: onCancel }, "Cancel"), submit));
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    submit.disabled = true;
    err.textContent = "";
    try { await onSubmit({ title: title?.value, body: body.value }); }
    catch (ex) { err.textContent = ex.message; submit.disabled = false; }
  });
  return form;
}

// ---- pages ----
async function feedPage() {
  const list = h("div");
  let page = 0;
  const tabs = h("div", { class: "tabs" },
    ...["active", "new", "top"].map((s) =>
      h("button", { "aria-pressed": String(state.sort === s), onclick: () => { state.sort = s; feedPage().then(show); } },
        s === "active" ? "Active" : s === "new" ? "New" : "Top")));
  const more = h("button", { class: "btn" }, "Load more");
  async function load() {
    const { threads, has_more } = await api("GET", `/api/threads?sort=${state.sort}&page=${page}`);
    if (page === 0 && !threads.length)
      list.append(h("div", { class: "empty" }, "No threads yet. ", h("a", { href: state.user ? "/new" : "/register", "data-link": true }, "Start the first one.")));
    for (const t of threads) list.append(threadCard(t));
    more.hidden = !has_more;
    page++;
  }
  more.addEventListener("click", load);
  await load();
  return h("div", {}, tabs, list, h("div", { class: "center" }, more));
}

function threadCard(t) {
  return h("article", { class: "card thread" },
    h("h2", {}, h("a", { href: `/t/${t.id}`, "data-link": true }, t.title)),
    t.body && h("p", { class: "body" }, t.body + (t.truncated ? "…" : "")),
    h("div", { class: "actions" },
      h("span", {}, `@${t.author}`), h("span", {}, ago(t.created_at)), likeButton(t, "threads"),
      h("a", { href: `/t/${t.id}`, "data-link": true }, `${t.reply_count} ${t.reply_count === 1 ? "reply" : "replies"}`)));
}

const MAX_DEPTH = 6; // deeper replies stay at the same indent so mobile screens don't run out of width

async function threadPage(id) {
  const { thread, replies } = await api("GET", `/api/threads/${id}`);
  document.title = `${thread.title} — spam`;
  const byParent = new Map();
  for (const r of replies) {
    const k = r.parent_id ?? 0;
    if (!byParent.has(k)) byParent.set(k, []);
    byParent.get(k).push(r);
  }
  const top = h("div", { class: "replies" });

  function renderLevel(parentId, depth) {
    const wrap = h("div");
    for (const r of byParent.get(parentId) || []) wrap.append(replyNode(r, depth));
    return wrap;
  }
  function replyNode(r, depth) {
    const kids = h("div");
    const box = h("div", { class: "reply" + (depth >= MAX_DEPTH ? " capped" : "") });
    if (r.deleted) box.append(h("div", { class: "deleted" }, "[deleted]"));
    else {
      const slot = h("div");
      box.append(
        h("div", { class: "meta" }, `@${r.author} · ${ago(r.created_at)}`),
        h("div", { class: "body" }, r.body),
        h("div", { class: "actions" },
          likeButton(r, "replies"),
          h("button", { class: "link", onclick: () => {
            if (!state.user) return go("/login");
            if (slot.firstChild) return slot.replaceChildren();
            slot.replaceChildren(composer({ placeholder: `Reply to @${r.author}`, submitLabel: "Reply", onCancel: () => slot.replaceChildren(),
              onSubmit: async ({ body }) => { await api("POST", `/api/threads/${id}/replies`, { body, parent_id: r.id }); render(); } }));
          } }, "Reply"),
          r.mine && h("button", { class: "link", onclick: async () => { if (confirm("Delete this reply?")) { await api("DELETE", `/api/replies/${r.id}`); render(); } } }, "Delete")),
        slot);
    }
    const next = depth >= MAX_DEPTH ? 0 : depth + 1;
    kids.append(renderLevel(r.id, next));
    box.append(kids);
    return box;
  }
  top.append(renderLevel(0, 0));

  const replyBox = state.user
    ? composer({ placeholder: "Add a reply…", submitLabel: "Reply", onSubmit: async ({ body }) => { await api("POST", `/api/threads/${id}/replies`, { body }); render(); } })
    : h("p", { class: "meta" }, h("a", { href: "/login", "data-link": true }, "Log in"), " to reply.");

  return h("div", {},
    h("article", { class: "card thread" },
      h("h2", {}, thread.title),
      thread.deleted ? h("p", { class: "deleted" }, "This thread was deleted.") : h("p", { class: "body" }, thread.body),
      h("div", { class: "actions" },
        !thread.deleted && [h("span", {}, `@${thread.author}`), h("span", {}, ago(thread.created_at)), likeButton(thread, "threads")],
        thread.mine && h("button", { class: "link", onclick: async () => { if (confirm("Delete this thread?")) { await api("DELETE", `/api/threads/${id}`); go("/"); } } }, "Delete"))),
    !thread.deleted && h("div", { class: "card" }, replyBox),
    top);
}

function newThreadPage() {
  if (!state.user) { go("/login"); return h("div"); }
  document.title = "New thread — spam";
  return h("div", { class: "card" }, h("h2", {}, "New thread"),
    composer({ withTitle: true, placeholder: "What's on your mind? (optional)", submitLabel: "Post",
      onSubmit: async ({ title, body }) => { const { id } = await api("POST", "/api/threads", { title, body }); go(`/t/${id}`); } }));
}

function authPage(mode) {
  document.title = `${mode === "login" ? "Log in" : "Sign up"} — spam`;
  const user = h("input", { placeholder: "Username", autocomplete: "username", required: true, maxlength: 24 });
  const pass = h("input", { type: "password", placeholder: "Password (8+ characters)", autocomplete: mode === "login" ? "current-password" : "new-password", required: true });
  const err = h("div", { class: "err" });
  const form = h("form", { class: "compose" }, user, pass, err,
    h("div", { class: "row" }, h("button", { class: "primary", type: "submit" }, mode === "login" ? "Log in" : "Create account")));
  form.addEventListener("submit", async (e) => {
    e.preventDefault();
    err.textContent = "";
    try {
      const r = await api("POST", `/api/${mode}`, { username: user.value, password: pass.value });
      state.user = r.user;
      renderNav();
      go("/");
    } catch (ex) { err.textContent = ex.message; }
  });
  return h("div", { class: "card" }, h("h2", {}, mode === "login" ? "Log in" : "Sign up"), form,
    h("p", { class: "meta" }, mode === "login" ? "New here? " : "Have an account? ",
      h("a", { href: mode === "login" ? "/register" : "/login", "data-link": true }, mode === "login" ? "Sign up" : "Log in")));
}

// ---- router ----
function show(node) { $app.replaceChildren(node); window.scrollTo(0, 0); }
let renderSeq = 0;
async function render() {
  const seq = ++renderSeq;
  const p = location.pathname;
  document.title = "spam — threads";
  try {
    let node;
    let m;
    if (p === "/") node = await feedPage();
    else if ((m = p.match(/^\/t\/(\d+)$/))) node = await threadPage(m[1]);
    else if (p === "/new") node = newThreadPage();
    else if (p === "/login") node = authPage("login");
    else if (p === "/register") node = authPage("register");
    else node = h("div", { class: "empty" }, "Nothing here. ", h("a", { href: "/", "data-link": true }, "Back to threads"));
    if (seq === renderSeq) show(node);
  } catch (e) {
    if (seq === renderSeq) show(h("div", { class: "empty" }, e.message));
  }
}

(async () => {
  try { state.user = (await api("GET", "/api/me")).user; } catch {}
  renderNav();
  render();
})();
