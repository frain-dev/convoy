### Create a subscription

```php[example]
$subscriptionData = [
    "name" => "event-sub",
    "app_id" => $appId,
    "endpoint_id" => $endpointId
];

$response = $convoy->subscriptions()->create($subscriptionData);
```

With the subscription in place, you're set to send an event.
