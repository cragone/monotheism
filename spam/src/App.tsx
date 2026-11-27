import { useCallback, useEffect, useState, type FormEvent } from "react";
import { Link, NavLink, Route, Routes, useNavigate, useParams } from "react-router-dom";
import { timeAgo, useApp, type Post, type User } from "./api";

function Avatar({ name, size = 10 }: { name: string; size?: number }) {
  const px = `${size * 0.25}rem`;
  const hue = [...name].reduce((a, c) => a + c.charCodeAt(0) * 7, 0) % 360;
  return (
    <div className="avatar avatar-placeholder">
      <div className="rounded-full text-primary-content" style={{ width: px, background: `oklch(70% 0.14 ${hue})` }}>
        <span className="font-bold">{name[0]?.toUpperCase()}</span>
      </div>
    </div>
  );
}

function Composer({ parentId, onPosted, placeholder }: { parentId?: number; onPosted: (p: Post) => void; placeholder?: string }) {
  const { call, me } = useApp();
  const [body, setBody] = useState("");
  const [err, setErr] = useState("");
  const [busy, setBusy] = useState(false);
  if (!me) return null;
  const submit = async (e: FormEvent) => {
    e.preventDefault();
    setBusy(true); setErr("");
    try {
      const p = await call<Post>("POST", "/api/posts", { body, parent_id: parentId ?? null });
      setBody(""); onPosted(p);
    } catch (x) { setErr((x as Error).message); } finally { setBusy(false); }
  };
  return (
    <form onSubmit={submit} className="flex gap-3 p-4 border-b border-base-300">
      <Avatar name={me.username} />
      <div className="flex-1">
        <textarea
          className="textarea textarea-ghost w-full resize-none px-0 focus:outline-none text-base"
          rows={2} maxLength={500} value={body} onChange={(e) => setBody(e.target.value)}
          placeholder={placeholder ?? "Start a thread…"}
        />
        {err && <p className="text-error text-sm">{err}</p>}
        <div className="flex items-center justify-between">
          <span className="text-xs opacity-50">{body.length}/500</span>
          <button className="btn btn-primary btn-sm rounded-full" disabled={busy || !body.trim()}>Post</button>
        </div>
      </div>
    </form>
  );
}

function PostCard({ post, onChange, onDelete, line }: { post: Post; onChange: (p: Post) => void; onDelete?: (id: number) => void; line?: boolean }) {
  const { call, me } = useApp();
  const nav = useNavigate();
  const toggleLike = async () => {
    if (!me) return;
    try { onChange(await call<Post>(post.liked ? "DELETE" : "PUT", `/api/posts/${post.id}/like`)); } catch { /* ignore */ }
  };
  const del = async () => {
    if (!confirm("Delete this post?")) return;
    await call("DELETE", `/api/posts/${post.id}`); onDelete?.(post.id);
  };
  return (
    <article className="flex gap-3 p-4 border-b border-base-300 hover:bg-base-200/40 transition-colors cursor-pointer"
      onClick={(e) => { if (!(e.target as HTMLElement).closest("a,button")) nav(`/p/${post.id}`); }}>
      <div className="flex flex-col items-center">
        <Avatar name={post.author.username} />
        {line && <div className="w-0.5 flex-1 bg-base-300 mt-2 rounded" />}
      </div>
      <div className="flex-1 min-w-0">
        <div className="flex items-baseline gap-2">
          <Link to={`/u/${post.author.username}`} className="font-semibold hover:underline">{post.author.display_name}</Link>
          <span className="text-sm opacity-50">@{post.author.username} · {timeAgo(post.created_at)}</span>
          {me?.pubkey === post.author.pubkey && (
            <button onClick={del} className="ml-auto text-xs opacity-40 hover:opacity-100 hover:text-error">delete</button>
          )}
        </div>
        <p className="whitespace-pre-wrap break-words mt-1">{post.body}</p>
        <div className="flex gap-5 mt-2 text-sm">
          <button onClick={toggleLike} className={`flex items-center gap-1 ${post.liked ? "text-primary" : "opacity-60 hover:opacity-100"}`}>
            {post.liked ? "♥" : "♡"} {post.likes || ""}
          </button>
          <Link to={`/p/${post.id}`} className="opacity-60 hover:opacity-100">💬 {post.replies || ""}</Link>
        </div>
      </div>
    </article>
  );
}

function PostList({ posts, setPosts, empty }: { posts: Post[]; setPosts: (f: (p: Post[]) => Post[]) => void; empty: string }) {
  if (!posts.length) return <p className="p-10 text-center opacity-50">{empty}</p>;
  return <>{posts.map((p) => (
    <PostCard key={p.id} post={p}
      onChange={(n) => setPosts((l) => l.map((x) => (x.id === n.id ? n : x)))}
      onDelete={(id) => setPosts((l) => l.filter((x) => x.id !== id))} />
  ))}</>;
}

function Home() {
  const { call, me, id } = useApp();
  const [tab, setTab] = useState<"all" | "following">("all");
  const [posts, setPosts] = useState<Post[]>([]);
  const [more, setMore] = useState(false);
  const load = useCallback(async (before?: number) => {
    const r = await call<Post[]>("GET", `/api/feed?tab=${tab}${before ? `&before=${before}` : ""}`);
    setPosts((p) => (before ? [...p, ...r] : r)); setMore(r.length === 30);
  }, [call, tab]);
  useEffect(() => { load().catch(() => {}); }, [load]);
  return (
    <>
      <div role="tablist" className="tabs tabs-border sticky top-0 bg-base-100/90 backdrop-blur z-10">
        <button role="tab" className={`tab flex-1 ${tab === "all" ? "tab-active" : ""}`} onClick={() => setTab("all")}>For you</button>
        <button role="tab" className={`tab flex-1 ${tab === "following" ? "tab-active" : ""}`} onClick={() => setTab("following")}>Following</button>
      </div>
      {id && !me && <ClaimProfile />}
      <Composer onPosted={(p) => setPosts((l) => [p, ...l])} />
      <PostList posts={posts} setPosts={setPosts} empty={tab === "following" ? "Follow people to see their threads here." : "Nothing here yet. Be the first to spam."} />
      {more && <button className="btn btn-ghost w-full" onClick={() => load(posts[posts.length - 1].id)}>Load more</button>}
    </>
  );
}

function ClaimProfile() {
  const { call, setMe } = useApp();
  const [username, setUsername] = useState("");
  const [err, setErr] = useState("");
  const submit = async (e: FormEvent) => {
    e.preventDefault(); setErr("");
    try { const r = await call<{ user: User }>("PUT", "/api/me", { username }); setMe(r.user); }
    catch (x) { setErr((x as Error).message); }
  };
  return (
    <form onSubmit={submit} className="p-4 border-b border-base-300 bg-base-200">
      <p className="font-semibold mb-2">Pick a username to start posting</p>
      <div className="join w-full">
        <span className="join-item btn btn-disabled">@</span>
        <input className="input join-item w-full" value={username} onChange={(e) => setUsername(e.target.value)} placeholder="username" maxLength={20} />
        <button className="btn btn-primary join-item">Claim</button>
      </div>
      {err && <p className="text-error text-sm mt-1">{err}</p>}
    </form>
  );
}

function ThreadPage() {
  const { pid } = useParams();
  const { call } = useApp();
  const [data, setData] = useState<{ post: Post; ancestors: Post[]; replies: Post[] } | null>(null);
  const [err, setErr] = useState("");
  const load = useCallback(() => call<typeof data>("GET", `/api/posts/${pid}`).then(setData).catch((e) => setErr(e.message)), [call, pid]);
  useEffect(() => { load(); }, [load]);
  const nav = useNavigate();
  if (err) return <p className="p-10 text-center opacity-60">{err}</p>;
  if (!data) return <div className="p-10 text-center"><span className="loading loading-dots" /></div>;
  const upd = (n: Post) => setData((d) => d && ({
    post: d.post.id === n.id ? n : d.post,
    ancestors: d.ancestors.map((x) => (x.id === n.id ? n : x)),
    replies: d.replies.map((x) => (x.id === n.id ? n : x)),
  }));
  return (
    <>
      <div className="p-3 border-b border-base-300 sticky top-0 bg-base-100/90 backdrop-blur z-10 flex items-center gap-3">
        <button className="btn btn-ghost btn-sm btn-circle" onClick={() => nav(-1)}>←</button>
        <span className="font-semibold">Thread</span>
      </div>
      {data.ancestors.map((a) => <PostCard key={a.id} post={a} onChange={upd} line />)}
      <div className="bg-base-200/60"><PostCard post={data.post} onChange={upd} onDelete={() => nav("/")} /></div>
      <Composer parentId={data.post.id} placeholder={`Reply to @${data.post.author.username}…`} onPosted={() => load()} />
      {data.replies.map((r) => <PostCard key={r.id} post={r} onChange={upd} onDelete={() => load()} />)}
    </>
  );
}

function ProfilePage() {
  const { username } = useParams();
  const { call, me, setMe } = useApp();
  const [tab, setTab] = useState<"threads" | "replies">("threads");
  const [d, setD] = useState<{ user: User; followers: number; following: number; is_following: boolean; is_me: boolean; posts: Post[] } | null>(null);
  const [err, setErr] = useState("");
  const [editing, setEditing] = useState(false);
  const load = useCallback(() => call<typeof d>("GET", `/api/users/${username}?tab=${tab}`).then(setD).catch((e) => setErr(e.message)), [call, username, tab]);
  useEffect(() => { load(); }, [load]);
  if (err) return <p className="p-10 text-center opacity-60">{err}</p>;
  if (!d) return <div className="p-10 text-center"><span className="loading loading-dots" /></div>;
  const follow = async () => {
    await call(d.is_following ? "DELETE" : "PUT", `/api/users/${d.user.username}/follow`); load();
  };
  return (
    <>
      <div className="p-5 border-b border-base-300">
        <div className="flex items-start gap-4">
          <Avatar name={d.user.username} size={16} />
          <div className="flex-1">
            <h1 className="text-xl font-bold">{d.user.display_name}</h1>
            <p className="opacity-50">@{d.user.username}</p>
          </div>
          {d.is_me
            ? <button className="btn btn-sm btn-outline rounded-full" onClick={() => setEditing(true)}>Edit profile</button>
            : me && <button className={`btn btn-sm rounded-full ${d.is_following ? "btn-outline" : "btn-primary"}`} onClick={follow}>{d.is_following ? "Following" : "Follow"}</button>}
        </div>
        {d.user.bio && <p className="mt-3 whitespace-pre-wrap">{d.user.bio}</p>}
        <p className="mt-3 text-sm opacity-60"><b>{d.followers}</b> followers · <b>{d.following}</b> following</p>
      </div>
      <div role="tablist" className="tabs tabs-border">
        <button role="tab" className={`tab flex-1 ${tab === "threads" ? "tab-active" : ""}`} onClick={() => setTab("threads")}>Threads</button>
        <button role="tab" className={`tab flex-1 ${tab === "replies" ? "tab-active" : ""}`} onClick={() => setTab("replies")}>Replies</button>
      </div>
      <PostList posts={d.posts} setPosts={(f) => setD((x) => x && { ...x, posts: f(x.posts) })} empty="Nothing yet." />
      {editing && <EditProfile user={d.user} onClose={() => setEditing(false)} onSaved={(u) => { setMe(u); setEditing(false); load(); }} />}
    </>
  );
}

function EditProfile({ user, onClose, onSaved }: { user: User; onClose: () => void; onSaved: (u: User) => void }) {
  const { call } = useApp();
  const [name, setName] = useState(user.display_name);
  const [bio, setBio] = useState(user.bio);
  const [err, setErr] = useState("");
  const save = async (e: FormEvent) => {
    e.preventDefault();
    try { const r = await call<{ user: User }>("PUT", "/api/me", { display_name: name, bio }); onSaved(r.user); }
    catch (x) { setErr((x as Error).message); }
  };
  return (
    <dialog className="modal modal-open">
      <form onSubmit={save} className="modal-box space-y-3">
        <h3 className="font-bold text-lg">Edit profile</h3>
        <input className="input w-full" value={name} onChange={(e) => setName(e.target.value)} maxLength={40} placeholder="Display name" />
        <textarea className="textarea w-full" value={bio} onChange={(e) => setBio(e.target.value)} maxLength={160} rows={3} placeholder="Bio" />
        {err && <p className="text-error text-sm">{err}</p>}
        <div className="modal-action"><button type="button" className="btn" onClick={onClose}>Cancel</button><button className="btn btn-primary">Save</button></div>
      </form>
    </dialog>
  );
}

function AccountPage() {
  const { id, me, create, recover, logout } = useApp();
  const [phrase, setPhrase] = useState("");
  const [err, setErr] = useState("");
  const [show, setShow] = useState(false);
  if (!id) {
    return (
      <div className="p-6 space-y-6">
        <div>
          <h1 className="text-2xl font-bold">Welcome to spam</h1>
          <p className="opacity-70 mt-1">There's no signup and no password. Your account is a secret key that lives on this device, backed up as a 24-word recovery phrase.</p>
        </div>
        <button className="btn btn-primary btn-lg w-full" onClick={create}>Create account</button>
        <div className="divider">or</div>
        <form className="space-y-2" onSubmit={(e) => { e.preventDefault(); try { recover(phrase); } catch (x) { setErr((x as Error).message); } }}>
          <label className="font-semibold">Recover an existing account</label>
          <textarea className="textarea w-full" rows={3} value={phrase} onChange={(e) => setPhrase(e.target.value)} placeholder="24 recovery words, separated by spaces" />
          {err && <p className="text-error text-sm">{err}</p>}
          <button className="btn w-full">Recover</button>
        </form>
      </div>
    );
  }
  return (
    <div className="p-6 space-y-5">
      <h1 className="text-2xl font-bold">Your account</h1>
      {me ? <p>Signed in as <Link className="link" to={`/u/${me.username}`}>@{me.username}</Link></p> : <p className="opacity-70">No username yet — claim one on the home page.</p>}
      <div className="alert alert-warning alert-soft block">
        <p className="font-semibold">Back up your recovery phrase.</p>
        <p className="text-sm">It's the only way to get this account back on another device. Anyone with it controls your account; we can't reset it.</p>
      </div>
      <button className="btn w-full" onClick={() => setShow((s) => !s)}>{show ? "Hide" : "Reveal"} recovery phrase</button>
      {show && (
        <ol className="grid grid-cols-3 gap-2 font-mono text-sm">
          {id.phrase.split(" ").map((w, i) => <li key={i} className="bg-base-200 rounded px-2 py-1"><span className="opacity-40 mr-1">{i + 1}</span>{w}</li>)}
        </ol>
      )}
      <p className="text-xs opacity-50 break-all">Public key: {id.pubkey}</p>
      <button className="btn btn-error btn-outline w-full" onClick={() => { if (confirm("Remove this account from this device? Make sure your phrase is saved.")) logout(); }}>Remove from this device</button>
    </div>
  );
}

export default function App() {
  const { me, id, ready } = useApp();
  const link = ({ isActive }: { isActive: boolean }) => `btn btn-ghost btn-circle text-xl ${isActive ? "text-primary" : "opacity-60"}`;
  return (
    <div className="min-h-screen flex justify-center bg-base-100">
      <div className="w-full max-w-xl border-x border-base-300 min-h-screen pb-16 relative">
        <header className="flex items-center justify-between p-3 border-b border-base-300">
          <Link to="/" className="text-2xl font-black tracking-tight text-primary">@spam</Link>
          {ready && !id && <Link to="/account" className="btn btn-primary btn-sm rounded-full">Get started</Link>}
        </header>
        <Routes>
          <Route path="/" element={<Home />} />
          <Route path="/p/:pid" element={<ThreadPage />} />
          <Route path="/u/:username" element={<ProfilePage />} />
          <Route path="/account" element={<AccountPage />} />
          <Route path="*" element={<p className="p-10 text-center opacity-60">Nothing here.</p>} />
        </Routes>
        <nav className="fixed bottom-0 w-full max-w-xl flex justify-around border-t border-base-300 bg-base-100/95 backdrop-blur p-1">
          <NavLink to="/" end className={link}>⌂</NavLink>
          {me && <NavLink to={`/u/${me.username}`} className={link}>☺</NavLink>}
          <NavLink to="/account" className={link}>⚙</NavLink>
        </nav>
      </div>
    </div>
  );
}
