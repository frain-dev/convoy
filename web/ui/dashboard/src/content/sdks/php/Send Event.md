### Sending an Event

To send an event, you'll need the `uid` from the application we created earlier.

```php[example]
$eventData = [
    "app_id" => $appId,
    "event_type" => "payment.success",
    "data" => [
        "event" => "payment.success",
        "data" => [
            "status" => "Completed",
            "description" => "Transaction Successful",
            "userID" => "test_user_id808"
        ]
    ]
];

$response = $convoy->events()->create($eventData);
```
