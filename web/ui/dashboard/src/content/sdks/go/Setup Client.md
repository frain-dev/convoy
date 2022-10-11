### Setup Client
```go[example]
import (
    convoy "github.com/frain-dev/convoy-go"
)

  c := convoy.New(convoy.Options{
      APIKey: "your_api_key",
  })
```

The SDK also supports authenticating via Basic Auth by providing your username and password

```go[example]
  c := convoy.New(convoy.Options{
      APIUsername: "default",
      APIPassword: "default",
  })
```

In the event you're using a self hosted convoy instance, you can define the url as part of what is passed into the `convoy.Options` struct

```go[example]
   c := convoy.New(convoy.Options{
       APIKey: "your_api_key",
       APIEndpoint: "self-hosted-instance",
   })
```

Now that your client has been configured, create a convoy application.
