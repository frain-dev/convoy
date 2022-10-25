### Create a subscription

```go[example]
subscription, err := c.Subscriptions.Create(&Convoy.CreateSubscriptionRequest{
    Name: "<subscription name>"
    AppID: app.UID
    EndpointID: "<endpoint-id>"
}, nil)

  if err != nil {
      log.Fatal("failed to create app endpoint \n", err)
  }
```

With the subscription in place, you're set to send an event.
