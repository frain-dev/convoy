# Basic Auth License + Feature Flag Gating — Manual Test Plan

## Prerequisites

- Convoy server built with latest changes
- A valid license key (Business plan with `oauth2_endpoint_auth` entitlement)
- Access to `convoy.json` or `mise.local.toml` to toggle the license key
- An outgoing project with at least one endpoint
- A webhook receiver (e.g., `http://localhost:9099/webhook`)

---

## Test Matrix

| # | License | Feature Flag | Expected Behavior |
|---|---------|-------------|-------------------|
| 1 | OFF | OFF | Lock tag + "Business plan" message; no form fields |
| 2 | OFF | ON (stale) | Lock tag + "Business plan" message; no form fields |
| 3 | ON | OFF | "Feature Flag Disabled" message; no form fields |
| 4 | ON | ON | Username/Password form fields shown; endpoint creates successfully |

---

## A. Frontend Tests

### A1. No License — Create Endpoint UI

**Setup:** Start server without a license key (`CONVOY_LICENSE_KEY="" go run ./cmd server`).

1. Navigate to an outgoing project → Endpoints → + Endpoint
2. Click "Auth" to expand the authentication section
3. Open the Authentication Type dropdown
4. **Verify:** All three options are visible (API Key, Basic Auth, OAuth2)
5. Select "Basic Auth"
6. **Verify:** A blue "Business" lock tag appears next to the dropdown
7. **Verify:** A message box appears: "Basic Auth Endpoint Authentication | Business — Basic Auth endpoint authentication is a Business plan feature. Please upgrade your license to use this feature."
8. **Verify:** No Username/Password fields are shown
9. Select "OAuth2"
10. **Verify:** Same "Business" lock tag and license-required message for OAuth2

### A2. No License — Early Adopter Features Page

**Setup:** Same as A1 (no license).

1. Navigate to Organisation Settings → Early Adopter Features
2. **Verify:** Page shows "No early adopter features available at this time."
3. **Verify:** mTLS, OAuth Token Exchange, and Basic Auth Endpoint are all hidden

### A3. License ON, Feature Flag OFF — Create Endpoint UI

**Setup:** Start server with a valid license key. Ensure Basic Auth Endpoint is toggled OFF in Early Adopter Features.

1. Navigate to Organisation Settings → Early Adopter Features
2. **Verify:** All 3 features are visible (mTLS, OAuth Token Exchange, Basic Auth Endpoint)
3. **Verify:** Basic Auth Endpoint toggle is OFF
4. Navigate to an outgoing project → Endpoints → + Endpoint
5. Click "Auth", open dropdown, select "Basic Auth"
6. **Verify:** No "Business" lock tag (license is valid)
7. **Verify:** Message: "Basic Auth Feature Flag Disabled — Basic Auth endpoint authentication feature flag is not enabled for your organization. Please enable it in Early Adopter Features settings."
8. **Verify:** No Username/Password fields shown

### A4. License ON, Feature Flag ON — Create Endpoint UI

**Setup:** Same as A3, then enable the Basic Auth Endpoint toggle.

1. Navigate to Organisation Settings → Early Adopter Features
2. Toggle ON "Basic Auth Endpoint"
3. **Verify:** Toggle turns blue, success notification appears
4. Navigate to an outgoing project → Endpoints → + Endpoint
5. Click "Auth", open dropdown, select "Basic Auth"
6. **Verify:** No lock tag, no warning messages
7. **Verify:** Username and Password form fields are shown
8. Fill in endpoint name, URL (`http://localhost:9099/webhook`), username (`testuser`), password (`testpass`)
9. Click "Create Endpoint"
10. **Verify:** Endpoint is created successfully

### A5. License ON, Feature Flag ON — Edit Endpoint with Basic Auth

1. Navigate to the endpoint created in A4
2. Click Edit
3. **Verify:** Auth section shows "Basic Auth" selected with Username pre-filled
4. Change the username to `updateduser`
5. Save
6. **Verify:** Update succeeds

### A6. Toggle Feature Flag OFF After Creating Endpoint

1. Navigate to Organisation Settings → Early Adopter Features
2. Toggle OFF "Basic Auth Endpoint"
3. Navigate back to Create Endpoint → Auth → Basic Auth
4. **Verify:** "Feature Flag Disabled" message appears again
5. **Verify:** Existing endpoints with Basic Auth still display in the endpoints list

---

## B. Backend / API Tests

### B1. No License — Create Endpoint via API

```bash
curl -X POST http://localhost:5005/api/v1/projects/{PROJECT_ID}/endpoints \
  -H "Authorization: Bearer {API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Basic Auth API Test",
    "url": "http://localhost:9099/webhook",
    "authentication": {
      "type": "basic_auth",
      "basic_auth": {
        "username": "apiuser",
        "password": "apipass"
      }
    }
  }'
```

**Expected:** `400` or error response with message containing "Basic Auth feature unavailable, please upgrade your license"

### B2. License ON, Feature Flag OFF — Create Endpoint via API

**Setup:** Start with license, ensure Basic Auth feature flag is OFF for the org.

Run the same curl as B1.

**Expected:** Endpoint is created but **without** Basic Auth config (silently ignored). Check the response — `authentication` should be `null` or absent. Server logs should contain: "Basic Auth configuration provided but feature flag not enabled, ignoring Basic Auth config"

### B3. License ON, Feature Flag ON — Create Endpoint via API

**Setup:** Enable Basic Auth in Early Adopter Features for the org.

Run the same curl as B1.

**Expected:** `201` success. Response includes `authentication.type: "basic_auth"` with the username.

### B4. License ON, Feature Flag ON — Update Endpoint via API

```bash
curl -X PUT http://localhost:5005/api/v1/projects/{PROJECT_ID}/endpoints/{ENDPOINT_ID} \
  -H "Authorization: Bearer {API_KEY}" \
  -H "Content-Type: application/json" \
  -d '{
    "name": "Updated Basic Auth Endpoint",
    "url": "http://localhost:9099/webhook",
    "authentication": {
      "type": "basic_auth",
      "basic_auth": {
        "username": "newuser",
        "password": "newpass"
      }
    }
  }'
```

**Expected:** `200` success with updated authentication.

### B5. No License — Update Endpoint via API

**Setup:** Restart without license.

Run the same curl as B4 on an existing endpoint.

**Expected:** Error response with "Basic Auth feature unavailable, please upgrade your license"

---

## C. Worker / Event Delivery Tests

### C1. Feature Flag ON — Event Delivery with Basic Auth

**Setup:** License ON, feature flag ON, endpoint with Basic Auth configured.

1. Start a webhook receiver that logs request headers:
   ```bash
   # Simple Python receiver
   python3 -c "
   from http.server import HTTPServer, BaseHTTPRequestHandler
   class H(BaseHTTPRequestHandler):
       def do_POST(self):
           print('Authorization:', self.headers.get('Authorization'))
           self.send_response(200)
           self.end_headers()
   HTTPServer(('', 9099), H).serve_forever()
   "
   ```
2. Send an event to the project targeting the Basic Auth endpoint
3. **Verify:** The webhook receiver logs an `Authorization: Basic <base64>` header
4. **Verify:** Decoding the base64 value gives `username:password`

### C2. Feature Flag OFF — Event Delivery with Basic Auth Endpoint

**Setup:** License ON, feature flag OFF, but endpoint still has Basic Auth in the database from a previous config.

1. Toggle OFF Basic Auth in Early Adopter Features
2. Start the agent (`go run ./cmd agent`)
3. Send an event targeting the Basic Auth endpoint
4. **Verify:** Server logs contain: "Endpoint has Basic Auth configured but feature flag is disabled, skipping Basic Auth authentication"
5. **Verify:** The webhook receiver does NOT receive an `Authorization` header

### C3. Nil Guard — Endpoint with BasicAuth type but null config

This is an edge case where the database has `authentication.type = 'basic_auth'` but `authentication.basic_auth` is null.

1. Manually update the endpoint in the database:
   ```sql
   UPDATE convoy.endpoints
   SET authentication = '{"type": "basic_auth"}'::jsonb
   WHERE uid = '{ENDPOINT_ID}';
   ```
2. Send an event targeting this endpoint
3. **Verify:** Server logs contain "Basic Auth config is nil" (not a panic/crash)
4. **Verify:** Event delivery proceeds without an Authorization header

---

## D. Edge Cases

### D1. Incoming Project — Auth Type Options

1. Navigate to an **incoming** project → Endpoints → + Endpoint
2. Click "Auth"
3. **Verify:** Only "API Key" is shown in the dropdown (Basic Auth and OAuth2 are hidden for incoming projects)

### D2. Non-Admin User — Feature Flag Toggle

1. Log in as a non-admin org member
2. Navigate to Early Adopter Features
3. **Verify:** Toggles are visible but disabled/non-functional (canManage = false)
4. **Verify:** Attempting to toggle does nothing

### D3. Switch Auth Type After Selecting Basic Auth (No License)

1. Start without a license
2. Create Endpoint → Auth → select Basic Auth (see lock message)
3. Switch back to API Key
4. **Verify:** Lock message disappears, API Key form fields appear normally
5. Switch to OAuth2
6. **Verify:** OAuth2 shows its own "Business" lock message

### D4. Concurrent License Change

1. Start with license, feature flag ON
2. Open Create Endpoint page, expand Auth, select Basic Auth (form fields visible)
3. In another terminal, restart server without license
4. Fill in username/password and click Create Endpoint
5. **Verify:** Backend rejects the request with license error (backend validates independently of frontend)

### D5. Empty Username/Password

1. License ON, feature flag ON
2. Create Endpoint → Auth → Basic Auth
3. Leave username and password empty
4. Fill in required fields (name, URL) and click Create
5. **Verify:** Endpoint either rejects with validation error or creates without auth config

---

## Test Completion Checklist

- [ ] A1: No license — UI shows Business lock + upgrade message
- [ ] A2: No license — Early Adopter page shows no features
- [ ] A3: License ON, flag OFF — UI shows feature flag disabled message
- [ ] A4: License ON, flag ON — UI shows form fields, endpoint creates
- [ ] A5: License ON, flag ON — Edit endpoint with Basic Auth works
- [ ] A6: Toggle OFF after creating — disabled message returns
- [ ] B1: No license — API returns license error
- [ ] B2: License ON, flag OFF — API silently ignores Basic Auth
- [ ] B3: License ON, flag ON — API creates with Basic Auth
- [ ] B4: License ON, flag ON — API updates Basic Auth
- [ ] B5: No license — API update returns license error
- [ ] C1: Flag ON — Event delivery includes Authorization header
- [ ] C2: Flag OFF — Event delivery skips Authorization header
- [ ] C3: Nil guard — No crash on null basic_auth config
- [ ] D1: Incoming project — Basic Auth not in dropdown
- [ ] D2: Non-admin — Toggles disabled
- [ ] D3: Auth type switching — Messages toggle correctly
- [ ] D4: Concurrent license change — Backend validates independently
- [ ] D5: Empty credentials — Validation works
