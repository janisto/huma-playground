---
name: pagination-endpoint
description: Create or review cursor-paginated Huma v2 list endpoints in huma-playground using its opaque cursor helper, strict validation, filtering, RFC 8288 Link headers, and pagination tests.
---

# Pagination endpoints

Read `AGENTS.md`, `internal/platform/pagination/`, and `internal/http/v1/items/` before editing cursor-paginated list
endpoints in the root Huma application.

## Scope and architecture

`pagination.Paginate` paginates an already ordered in-memory slice. It is appropriate for this playground's static item
example and other small bounded collections. It is not a database pagination abstraction.

For Firestore or another persistent store, paginate in the service or repository using a stable deterministic order,
the datastore's cursor primitives, and `limit + 1`. Do not load an unbounded collection merely to reuse the slice
helper. Keep opaque HTTP cursor encoding separate from storage cursors when their contracts differ.

## Input and handler pattern

Embed `pagination.Params`; it defines an opaque cursor bounded to 2048 characters and a limit defaulting to 20 with a
maximum of 100:

```go
type ResourceListInput struct {
	pagination.Params
	Category string `query:"category" doc:"Filter by category" enum:"active,inactive"`
}
```

Decode the cursor, require the exact endpoint-specific type for every non-empty cursor, apply filters, and reject a
cursor whose referenced item is absent from the filtered result:

```go
cursor, err := pagination.DecodeCursor(input.Cursor)
if err != nil {
	return nil, huma.Error400BadRequest("invalid cursor format")
}
if input.Cursor != "" && cursor.Type != resourceCursorType {
	return nil, huma.Error400BadRequest("cursor type mismatch")
}

filtered := filterResources(resources, input.Category)
if cursor.Value != "" && findResourceIndex(filtered, cursor.Value) == -1 {
	return nil, huma.Error400BadRequest("cursor references unknown item")
}
```

Preserve active filters. `Paginate` adds the effective limit and replaces the cursor itself. Build Link paths from the
explicit configured API prefix:

```go
query := url.Values{}
if input.Category != "" {
	query.Set("category", input.Category)
}

result := pagination.Paginate(
	filtered,
	cursor,
	input.DefaultLimit(),
	resourceCursorType,
	func(resource Resource) string { return resource.ID },
	prefix+"/resources",
	query,
)
```

Return `result.LinkHeader` through a typed `header:"Link"` output field. An empty value emits no useful relation.
Malformed, cross-endpoint, and stale cursors return 400. Limit and enum validation failures return 422 through Huma.
Cursor values are opaque URL-safe Base64; clients must not construct or interpret them.

## Verification

Cover first, middle, and last pages; empty results; default, minimum, and maximum limits; stable ordering; preserved
filters and effective limits; next and previous relations; malformed, wrong-type, unknown, and oversized cursors; and
JSON or CBOR responses. Assert decoded page contents as well as status and Link relations.

Run focused package tests, then `just build`, `just test`, and `just lint`.
