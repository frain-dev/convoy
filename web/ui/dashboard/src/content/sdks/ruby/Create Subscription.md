### Create a subscription

```console[example]
curl --request POST \
  --url https://dashboard.getconvoy.io/api/v1/subscriptions \
  --header 'Authorization: Bearer <api-key>' \
  --header 'Content-Type: application/json' \
  --data '{
  "app_id": "<your app ID>",
  "endpoint_id": "<your endpoint ID>",
  "name": "Subscription name"
}'
```

With the subscription in place, you're set to send an event.
