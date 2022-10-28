### Create a subscription

```js[example]try {
  const subscriptionData = {
    "name": "event-sub",
    "app_id": appId,
    "endpoint_id": endpointId,
  };

  const response = await convoy.subscriptions.create(subscriptionData);
} catch (error) {
  console.log(error);
}
```

With the subscription in place, you're set to send an event.
