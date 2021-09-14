---
title: Apps
description: 'Your list of apps on Convoys'
id: apps
---

# Apps

Apps on Convoy represents the events receivers, in some contexts your users. Convoy exposes enpoints that enables you to add, remove, edit and update apps. An app (a user) can psosibly have multiple endpoint to listen to events from.

### Create App

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

### Get Apps

<div class="code-snippet">
    <div class="code-snippet--details">
        <img src="../link-icon.svg" alt="link icon">
        <div class="code-snippet--url">/apps</div>
    </div>
    <div class="code-snippet--method get">GET</div>
</div>

```json[Reponse]
{
    "uid": "878787sf7f878s78sfsdhhj",
    "name": "Test Name",
    "org_id": 98398983,
    "secret": "secret_key"
}
```
