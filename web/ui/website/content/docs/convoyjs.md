---
title: Convoy.js SDK
description: 'Convoy.js SDK Configuration'
id: convoy.js
---


#### Installation
Install convoy.js with

```bash
$ npm install convoy.js
```

#### Setup Client

Next, require the `convoy` module and setup with your auth credentials.

```json[]
    const { Convoy } = require('convoy.js');
    const convoy = new Convoy({ api_key: 'your_api_key' })
```

The SDK also supports authenticating via Basic Auth by defining your username and password.

```json[]
    const convoy = new Convoy({ username: 'default', password: 'default' })
```

In the event you're using a self hosted convoy instance, you can define the url as part of what is passed into convoy's constructor.
```json[]
    const convoy = new Convoy({ api_key: 'your_api_key', uri: 'self-hosted-instance' })
```
