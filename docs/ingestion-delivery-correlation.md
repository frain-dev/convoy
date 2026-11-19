# Correlate upstream webhook delivery attempts

For each HTTP source you want to trace, configure the source mask ID and the
name of its sender's delivery GUID header on the Convoy HTTP deployment.
Keep this mapping in deployment configuration so it is available before
Convoy can read from PostgreSQL.

```json
{
  "ingest_delivery_id_headers": {
    "source-mask-id": "X-GitHub-Delivery"
  }
}
```

The equivalent environment variable is
`CONVOY_INGEST_DELIVERY_ID_HEADERS=source-mask-id:X-GitHub-Delivery`. Use a
comma to separate entries. Choose a header that carries a delivery GUID and
persists across retries. Keep the mapping current when creating or replacing
a source. Source idempotency configuration continues to govern deduplication.

With a mapping configured, the Convoy server or agent can respond with HTTP
503 during startup if PostgreSQL is unreachable. The response has a request
ID and does not acknowledge the webhook. The listener reports unavailable
until the regular startup path takes over. Requests must reach the listener
for Convoy to log them. An ingress that routes only to ready targets may
return the 503 upstream instead.

Set `CONVOY_LOGGER_LEVEL=info` to capture accepted attempts. Search the
structured logs for `upstream_delivery_id`. A rejected attempt
and a later accepted retry have different `request_id` values. An accepted
entry includes the Convoy `event_id` for following asynchronous processing.
The accepted entry means Convoy queued the event.
