const API = process.env.NEXT_PUBLIC_API_URL || "http://localhost:8080";

// createLink POSTs a long URL to the backend and returns {code, url, short_url}.
export async function createLink(url) {
  const res = await fetch(`${API}/api/links`, {
    method: "POST",
    headers: { "Content-Type": "application/json" },
    body: JSON.stringify({ url }),
  });
  if (!res.ok) throw new Error((await res.json()).error || "failed");
  return res.json();
}

// listLinks fetches all links with their click counts (no cache — always fresh).
export async function listLinks() {
  const res = await fetch(`${API}/api/links`, { cache: "no-store" });
  if (!res.ok) throw new Error("failed to load links");
  return res.json();
}

// getStats fetches analytics for one code: total, daily counts, recent clicks.
export async function getStats(code) {
  const res = await fetch(`${API}/api/links/${code}/stats`, { cache: "no-store" });
  if (!res.ok) throw new Error("failed to load stats");
  return res.json();
}
