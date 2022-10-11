### Setup Client

Next, require the `convoy` module and setup with your auth credentials.

```js[example]
const { Convoy } = require("convoy.js");
const convoy = new Convoy({ api_key: "your_api_key" });
```

The SDK also supports authenticating via Basic Auth by defining your username and password.

```js[example]
const convoy = new Convoy({ username: "default", password: "default" });
```

In the event you're using a self hosted convoy instance, you can define the url as part of what is passed into convoy's constructor.

```js[example]
const convoy = new Convoy({
  api_key: "your_api_key",
  uri: "self-hosted-instance",
});
```
