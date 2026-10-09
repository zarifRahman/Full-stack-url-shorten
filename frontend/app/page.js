"use client";

import { useEffect, useState } from "react";
import { createLink, listLinks } from "../lib/api";

// Home is the dashboard page: a form to shorten a URL and a table of existing
// links with click counts, copy buttons, and links to each analytics page.
export default function Home() {
  const [url, setUrl] = useState("");
  const [links, setLinks] = useState([]);
  const [error, setError] = useState("");
  const [loading, setLoading] = useState(false);

  // refresh reloads the link list from the backend into state.
  async function refresh() {
    try {
      setLinks(await listLinks());
    } catch (e) {
      setError(e.message);
    }
  }

  useEffect(() => {
    refresh();
  }, []);

  // onSubmit creates a short link from the form input, then refreshes the list.
  async function onSubmit(e) {
    e.preventDefault();
    setError("");
    setLoading(true);
    try {
      await createLink(url);
      setUrl("");
      await refresh();
    } catch (e) {
      setError(e.message);
    } finally {
      setLoading(false);
    }
  }

  // copy writes the given text (a short URL) to the clipboard.
  function copy(text) {
    navigator.clipboard?.writeText(text);
  }

  return (
    <>
      <div className="card">
        <h1>Shorten a URL</h1>
        <form onSubmit={onSubmit}>
          <input
            type="text"
            placeholder="https://example.com/very/long/link"
            value={url}
            onChange={(e) => setUrl(e.target.value)}
            required
          />
          <button disabled={loading}>{loading ? "..." : "Shorten"}</button>
        </form>
        {error && <p className="err">{error}</p>}
      </div>

      <div className="card">
        <h2>Your links</h2>
        {links.length === 0 ? (
          <p className="muted">No links yet. Create one above.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>Short</th>
                <th>Original</th>
                <th>Clicks</th>
                <th></th>
              </tr>
            </thead>
            <tbody>
              {links.map((l) => (
                <tr key={l.code}>
                  <td>
                    <a href={l.short_url} target="_blank" rel="noreferrer">
                      /{l.code}
                    </a>
                  </td>
                  <td className="truncate muted" title={l.url}>{l.url}</td>
                  <td>{l.clicks}</td>
                  <td>
                    <button className="copy" onClick={() => copy(l.short_url)}>
                      Copy
                    </button>{" "}
                    <a href={`/analytics/${l.code}`}>Analytics</a>
                  </td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
