# Repair payload viewing and make event search explicit

The event payload viewer displays JSON keys without quotes and lacks a way to
copy the entire payload. Preserve valid JSON formatting and provide an accessible
copy action that works when the viewer is collapsed. Preserve string escapes and
handle clipboard failures without a false success message.

Replace ambiguous event searching with one input and a dropdown on its leading
search control. Provide these modes in order: Event metadata, JSON payload,
Idempotency key, Idempotency value. Use a magnifier for metadata and distinct icons
for the other modes, accessible mode names, and mode-specific placeholders/help.
Retain hover/focus access to copying the full idempotency key in event details.

Metadata input must remain literal text even when it resembles JSON. Payload
search must accept a JSON object or shorthand such as `status: "paid"` and
`data: {status: "paid"}`, normalizing to valid JSON without rewriting strings.
Reject invalid/empty/non-object payload input visibly while keeping the previous
applied results. Preserve PostgreSQL JSON containment and nesting semantics.

Key and raw provider value searches are distinct exact lookups. Keep key before
value in the dropdown. Changing modes clears the prior search but retains date
and source filters. URLs must restore the chosen mode and query after reload.
Preserve legacy API JSON detection when no explicit text mode is requested,
existing combined query/body semantics, entitlements, and portal restrictions.
Reject unsupported explicit query modes and conflicting key/value lookup filters.

Update help and public API documentation to match the behavior. Add meaningful
regression tests and validate the real browser flows against a running disposable
backend using synthetic records. Report what ran and what remains unverified;
do not claim browser validation from component tests or mocks alone.
