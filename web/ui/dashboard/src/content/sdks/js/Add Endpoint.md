### Create Endpoint

After setting up Convoy, you'll need to create an endpoint. An endpoint represents a target URL to receive events.

```js[example]
try {
  const endpointData = {
    url: "https://0d87-102-89-2-172.ngrok.io",
    description: "Default Endpoint",
    secret: "endpoint-secret",
    events: ["*"],
  };

  const response = await convoy.endpoints.create(endpointData);
} catch (error) {
  console.log(error);
}
```
