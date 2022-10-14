## Send an event

To send an event, you'll need the `uid` from the application you created earlier.

```terminal[console]
curl --request POST \
  --url https://dashboard.getconvoy.io/api/v1/events \
  --header 'Authorization: Bearer <api-key>' \
  --header 'Content-Type: application/json' \
  --data '{
    "app_id": "<app-id>",
    "event_type": "payment.success",
    "data": {
      "event": "payment.success",
      "data": {
        "status": "Completed",
        "description": "Transaction Successful",
        "userID": "test_user_id808"
      }
    }
}'
```
