"use client";

import { useEffect, useState } from "react";
import {
  LineChart,
  Line,
  XAxis,
  YAxis,
  Tooltip,
  CartesianGrid,
  ResponsiveContainer,
} from "recharts";
import { getStats } from "../../../lib/api";

// Analytics is the per-link stats page: total clicks, a 30-day line chart, and
// a table of recent clicks. Reads the code from the route param.
export default function Analytics({ params }) {
  const { code } = params;
  const [stats, setStats] = useState(null);
  const [error, setError] = useState("");

  useEffect(() => {
    getStats(code).then(setStats).catch((e) => setError(e.message));
  }, [code]);

  if (error) return <div className="card err">{error}</div>;
  if (!stats) return <div className="card muted">Loading…</div>;

  return (
    <>
      <p>
        <a href="/">← Back</a>
      </p>

      <div className="card">
        <h1>/{stats.code}</h1>
        <p className="muted truncate" title={stats.url}>→ {stats.url}</p>
        <div className="big">{stats.total}</div>
        <p className="muted">total clicks</p>
      </div>

      <div className="card">
        <h2>Clicks (last 30 days)</h2>
        {stats.daily.length === 0 ? (
          <p className="muted">No clicks yet.</p>
        ) : (
          <ResponsiveContainer width="100%" height={260}>
            <LineChart data={stats.daily}>
              <CartesianGrid strokeDasharray="3 3" />
              <XAxis dataKey="date" />
              <YAxis allowDecimals={false} />
              <Tooltip />
              <Line type="monotone" dataKey="count" stroke="#4f46e5" strokeWidth={2} />
            </LineChart>
          </ResponsiveContainer>
        )}
      </div>

      <div className="card">
        <h2>Recent clicks</h2>
        {stats.recent.length === 0 ? (
          <p className="muted">No clicks yet.</p>
        ) : (
          <table>
            <thead>
              <tr>
                <th>When</th>
                <th>Referer</th>
                <th>User agent</th>
              </tr>
            </thead>
            <tbody>
              {stats.recent.map((c, i) => (
                <tr key={i}>
                  <td>{new Date(c.created_at).toLocaleString()}</td>
                  <td className="truncate muted">{c.referer || "—"}</td>
                  <td className="truncate muted">{c.user_agent || "—"}</td>
                </tr>
              ))}
            </tbody>
          </table>
        )}
      </div>
    </>
  );
}
