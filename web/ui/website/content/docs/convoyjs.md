---
title: Convoy.js SDK
description: "Convoy.js SDK Configuration"
id: convoy.js
order: 6
---

### Installation

Install convoy.js with

```bash
$ npm install convoy.js
```

#### Setup Client

Next, require the `convoy` module and setup with your auth credentials.

```js
const { Convoy } = require("convoy.js");
const convoy = new Convoy({ api_key: "your_api_key" });
```

The SDK also supports authenticating via Basic Auth by defining your username and password.

```js
const convoy = new Convoy({ username: "default", password: "default" });
```

In the event you're using a self hosted convoy instance, you can define the url as part of what is passed into convoy's constructor.

```js
const convoy = new Convoy({
  api_key: "your_api_key",
  uri: "self-hosted-instance",
});
```

### Creating an Application

An application represents a user's application trying to receive webhooks. Once you create an application, you'll receive a `uid` as part of the response that you should save and supply in subsequent API calls to perform other requests such as creating an event.

```js
try {
  const appData = { name: "my_app", support_email: "support@myapp.com" };

  const response = await convoy.application.create(appData);

  const appId = response.data.uid;
} catch (error) {
  console.log(error);
}
```

### Add Application Endpoint

After creating an application, you'll need to add an endpoint to the application you just created. An endpoint represents a target URL to receive events.

```js
try {
  const endpointData = {
    url: "https://0d87-102-89-2-172.ngrok.io",
    description: "Default Endpoint",
    secret: "endpoint-secret",
    events: ["*"],
  };

  const response = await convoy.endpoints.create(appId, endpointData);
} catch (error) {
  console.log(error);
}
```

### Sending an Event

To send an event, you'll need the `uid` from the application we created earlier.

```js
try {
  const eventData = {
    app_id: appId,
    event_type: "payment.success",
    data: {
      event: "payment.success",
      data: {
        status: "Completed",
        description: "Transaction Successful",
        userID: "test_user_id808",
      },
    },
  };

  const response = await convoy.events.create(eventData);
} catch (error) {
  console.log(error);
}
```