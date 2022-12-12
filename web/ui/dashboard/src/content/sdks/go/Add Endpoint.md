### Create Endpoint

After setting up Convoy, you'll need to create an endpoint. An endpoint represents a target URL to receive events.

```go[example]
endpoint, err := c.Endpoints.Create(&Convoy.CreateEndpointRequest{
    Name: "Endpoint name",
    URL: "http://localhost:8081",
    Description: "Some description",
}, nil)

  if err != nil {
      log.Fatal("failed to create endpoint \n", err)
  }
```
