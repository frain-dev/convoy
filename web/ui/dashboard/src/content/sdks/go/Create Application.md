### Creating an Application

An application represents a user's application trying to receive webhooks. Once you create an application, you'll receive a `uid` as part of the response that you should save and supply in subsequent API calls to perform other requests such as creating an event.

```go[example]
  app, err := c.Applications.Create(&convoy.CreateApplicationRequest{
      Name: "My_app",
      SupportEmail: "support@myapp.com",
  }, nil)

  if err != nil {
      log.Fatal("failed to create app \n", err)
  }
```
