### Add Application Endpoint

After creating an application, you'll need to add an endpoint to the application you just created. An endpoint represents a target URL to receive events.

```go[example]
endpoint, err := c.Endpoints.Create(app.UID, &Convoy.CreateEndpointRequest{
    URL: "http://localhost:8081",
    Description: "Some description",
}, nil)

  if err != nil {
      log.Fatal("failed to create app endpoint \n", err)
  }
```
