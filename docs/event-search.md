# Event search

The Events log has one search field. Use the dropdown on its search icon to
choose a search type:

The control at the start of the field shows a magnifier for metadata, braces
for payload, a key for the stored idempotency key, or a hash for the raw
idempotency value. Hover or focus it for the mode name, or open the dropdown
to switch modes. Each mode has its own placeholder and help text.

- **Event metadata:** Find an event ID prefix or a match in its event type,
  source name, or idempotency key. Input is treated as text, including braces.
- **JSON payload:** Enter fields such as `status: "paid"` or
  `data: {status: "paid"}`. A complete JSON object works too. Outer braces and
  quotes around simple field names are optional; string values need double
  quotes. All supplied fields must match the payload at the same nesting level.
  The dashboard normalizes the input to JSON before sending it. Invalid input shows a
  validation message and keeps the last successfully applied results.
- **Idempotency key:** Enter the exact stored key shown in event details.
  This is a separate lookup from the raw value received from a provider.
- **Idempotency value:** Enter the exact raw value received at a source's
  idempotency key location. Use the source filter if several sources qualify.
  This value can differ from the stored idempotency key shown in event details.

Switching search type clears the previous search. Date and source filters stay
selected. The selected date window applies to every search type.

Metadata and JSON payload search require the Event Search entitlement.

## API

For `GET /events`, use these query parameters:

- `query=invoice.paid&queryMode=text` searches metadata without interpreting
  JSON in the query.
- `body={"status":"paid"}` searches payload containment. URL-encode the JSON.
- `idempotencyValue=provider-delivery-123` looks up the raw provider value.
- `idempotencyKey=stored-key` looks up the stored key exactly.

Existing requests that omit `queryMode` retain automatic JSON detection in
`query`. An explicit `query` and `body` can still be combined as an AND filter.
Other `queryMode` values are rejected. Provider lookup cannot be combined with
`idempotencyKey` and is unavailable to portal links.

The event payload viewer preserves valid JSON formatting. Its Copy button
copies the complete formatted payload even while the viewer is collapsed.
Hovering over the idempotency key reveals its Copy control and full-value
tooltip; the control is also available on keyboard focus.
