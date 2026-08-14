package postgres

import (
	"encoding/json"
	"html/template"
	"net/http"
	"strings"
)

var monitorTmpl = template.Must(template.New("queue-monitor").Parse(`<!DOCTYPE html>
<html lang="en">
<head>
  <meta charset="utf-8">
  <title>Convoy queue</title>
  <style>
    body { font-family: ui-sans-serif, system-ui, sans-serif; margin: 24px; color: #111; }
    h1 { font-size: 1.25rem; }
    table { border-collapse: collapse; width: 100%; }
    th, td { text-align: left; padding: 8px 12px; border-bottom: 1px solid #e5e5e5; }
    th { color: #555; font-weight: 600; }
    .empty { color: #666; margin-top: 16px; }
  </style>
</head>
<body>
  <h1>Postgres queue</h1>
  {{if .Counts}}
  <table>
    <thead>
      <tr><th>Queue</th><th>Pending</th><th>Processing</th><th>Archived</th></tr>
    </thead>
    <tbody>
      {{range .Counts}}
      <tr>
        <td>{{.QueueName}}</td>
        <td>{{.Pending}}</td>
        <td>{{.Processing}}</td>
        <td>{{.Archived}}</td>
      </tr>
      {{end}}
    </tbody>
  </table>
  {{else}}
  <p class="empty">No jobs in convoy.queue_jobs.</p>
  {{end}}
</body>
</html>`))

func (q *PostgresQueue) Monitor() http.Handler {
	return q.MonitorWithRootPath("/queue/monitoring")
}

func (q *PostgresQueue) MonitorWithRootPath(_ string) http.Handler {
	return http.HandlerFunc(q.serveMonitor)
}

func (q *PostgresQueue) serveMonitor(w http.ResponseWriter, r *http.Request) {
	counts, err := q.Counts(r.Context())
	if err != nil {
		http.Error(w, "failed to load queue counts", http.StatusInternalServerError)
		return
	}

	if wantsJSON(r) {
		w.Header().Set("Content-Type", "application/json")
		_ = json.NewEncoder(w).Encode(map[string]any{"queues": counts})
		return
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = monitorTmpl.Execute(w, map[string]any{"Counts": counts})
}

func wantsJSON(r *http.Request) bool {
	if strings.EqualFold(r.URL.Query().Get("format"), "json") {
		return true
	}
	if strings.Contains(r.Header.Get("Accept"), "application/json") {
		return true
	}
	path := r.URL.Path
	return strings.HasSuffix(path, "/json") || strings.HasSuffix(path, ".json")
}
