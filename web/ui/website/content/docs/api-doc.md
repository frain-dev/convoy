---
title: HTTP API
description: Consul exposes a RESTful HTTP API to control almost every aspect of the Consul agent.
---

# HTTP API Structure

The main interface to Consul is a RESTful HTTP API. The API can perform basic
CRUD operations on nodes, services, checks, configuration, and more.

## Authentication

When authentication is enabled, a Consul token should be provided to API
requests using the `X-Consul-Token` header or with the
Bearer scheme in the authorization header.
This reduces the probability of the
token accidentally getting logged or exposed. When using authentication,
clients should communicate via TLS. If you don’t provide a token in the request, then the agent default token will be used.

<div class="code-snippet">
    <div class="code-snippet--details">
        <img src="../link-icon.svg" alt="link icon">
        <div class="code-snippet--url">/apps</div>
    </div>
    <div class="code-snippet--method post">POST</div>
</div>

```json
{
    "org_id": 98398983,
    "name": "Test Name",
    "secret": "secret_key"
}
```

```json[Reponse]
{
    "uid": "878787sf7f878s78sfsdhhj",
    "name": "Test Name",
    "org_id": 98398983,
    "secret": "secret_key"
}
```
