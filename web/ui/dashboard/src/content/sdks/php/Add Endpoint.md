### Create Endpoint

After setting up Convoy, you'll need to create an endpoint. An endpoint represents a target URL to receive events.

```php[example]
$endpointData = [
    name: "Endpoint name",
    "url" => "https://0d87-102-89-2-172.ngrok.io",
    "description" => "Default Endpoint",
    "secret" => "endpoint-secret",
    "events" => ["*"]
]

$response = $convoy->endpoints()->create($endpointData);
```
