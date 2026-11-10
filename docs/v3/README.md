# OpenAPI 3 generation

Run `make generate_docs` or `mise run gen/docs`; both generate Swagger from Go
annotations, convert it to OpenAPI 3, and apply the same response corrections.
Do not edit the generated specifications directly.

`GetDeliveryAttempts` returns an array by default and a page with `content` and
`next_cursor` when `paginated=true`. Swagger 2 cannot express this union. The
handler's `@x-convoy-paginated-response true` annotation therefore retains the
legacy array schema in Swagger, and `docs/fix_openapi3_spec.jq` expands it to an
OpenAPI 3 `oneOf` after conversion. The page reuses the generated array schema,
so its item fields stay in sync with the handler's response annotation.

`docs/test_openapi3_spec.sh` checks both branches, required page fields, YAML/JSON
agreement for the operation, idempotence, and rejection of unexpected source
shapes. Both generation entry points run these checks before spec validation.
