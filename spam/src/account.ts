// Account model copied from super-chat: no signup/login. The account is 32
// random bytes that live only on this device, shown as a 24-word recovery
// phrase. An Ed25519 key derived from it signs every request.
import { ed25519 } from "@noble/curves/ed25519";
import { sha256 } from "@noble/hashes/sha2";
import { bytesToHex, utf8ToBytes } from "@noble/hashes/utils";
import { entropyToMnemonic, mnemonicToEntropy, validateMnemonic } from "@scure/bip39";
import { wordlist } from "@scure/bip39/wordlists/english";

const STORE = "spam.seed.v1";

export type Identity = { seed: Uint8Array; priv: Uint8Array; pubkey: string; phrase: string };

function derive(seed: Uint8Array): Identity {
  const priv = sha256(new Uint8Array([...utf8ToBytes("spam/ed25519/v1"), ...seed]));
  return {
    seed,
    priv,
    pubkey: bytesToHex(ed25519.getPublicKey(priv)),
    phrase: entropyToMnemonic(seed, wordlist),
  };
}

export function loadIdentity(): Identity | null {
  try {
    const h = localStorage.getItem(STORE);
    if (!h || h.length !== 64) return null;
    const seed = Uint8Array.from(h.match(/../g)!.map((b) => parseInt(b, 16)));
    return derive(seed);
  } catch {
    return null;
  }
}

function save(id: Identity) {
  localStorage.setItem(STORE, bytesToHex(id.seed));
}

export function createIdentity(): Identity {
  const id = derive(crypto.getRandomValues(new Uint8Array(32)));
  save(id);
  return id;
}

export function importPhrase(phrase: string): Identity {
  const p = phrase.trim().toLowerCase().split(/\s+/).join(" ");
  if (!validateMnemonic(p, wordlist)) throw new Error("That recovery phrase isn't valid.");
  const seed = mnemonicToEntropy(p, wordlist);
  if (seed.length !== 32) throw new Error("Recovery phrase must be 24 words.");
  const id = derive(seed);
  save(id);
  return id;
}

export function forgetIdentity() {
  localStorage.removeItem(STORE);
}

export async function signedFetch(id: Identity | null, method: string, path: string, body?: unknown): Promise<Response> {
  const headers: Record<string, string> = {};
  const payload = body === undefined ? "" : JSON.stringify(body);
  if (body !== undefined) headers["Content-Type"] = "application/json";
  if (id) {
    const ts = Math.floor(Date.now() / 1000).toString();
    const nonce = bytesToHex(crypto.getRandomValues(new Uint8Array(8)));
    const msg = `${method}\n${path}\n${ts}\n${nonce}\n${bytesToHex(sha256(utf8ToBytes(payload)))}`;
    headers["X-Spam-Nonce"] = nonce;
    headers["X-Spam-Key"] = id.pubkey;
    headers["X-Spam-Ts"] = ts;
    headers["X-Spam-Sig"] = bytesToHex(ed25519.sign(utf8ToBytes(msg), id.priv));
  }
  return fetch(path, { method, headers, body: body === undefined ? undefined : payload });
}
