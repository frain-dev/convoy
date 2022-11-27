### Create Endpoint

After setting up Convoy, you'll need to create an endpoint. An endpoint represents a target URL to receive events.

```ruby[example]
endpoint = Convoy::Endpoint.new(
  data: {
    "description": "Endpoint One",
    "http_timeout": "1m",
    "url": "https://webhook.site/73932854-a20e-4d04-a151-d5952e873abd"
  }
)

endpoint_response = endpoint.save
```
