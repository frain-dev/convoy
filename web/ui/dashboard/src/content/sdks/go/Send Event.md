### Sending an Event

To send an event, you'll need the `uid` from the application we created earlier.

```go[example]
event, err := c.Events.Create(&convoy.CreateEventRequest{
		AppID:     app.UID,
		EventType: "test.customer.event",
		Data:      []byte(`{"event_type": "test.event", "data": { "Hello": "World", "Test": "Data" }}`),
	}, nil)

	if err != nil {
		log.Fatal("failed to create app event \n", err)
	}
```
