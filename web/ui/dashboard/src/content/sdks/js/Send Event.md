### Sending an Event

To send an event, you'll need the `uid` from the endpoint we created earlier.

```js[example]
try {
  const eventData = {
    endpoint_id: endpointId,
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
