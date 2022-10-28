### Create a subscription

```python[example]
subscription_data = {
  "name": "event-sub",
  "app_id": app_id,
  "endpoint_id": endpoint_id,
}

(response, status) = convoy.subscription.create({}, subscription_data)
```

With the subscription in place, you're set to send an event.
