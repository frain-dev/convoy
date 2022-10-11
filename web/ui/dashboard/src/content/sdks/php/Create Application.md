### Creating an Application

An application represents a user's application trying to receive webhooks. Once you create an application, you'll receive a `uid` from the response that you should save and supply in subsequent API calls to perform other requests such as creating an event.

```php[example]
$appData = ["name" => "my_app", "support_email" => "support@myapp.com"];

$response = $convoy->applications()->create($appData);

$appId = $response['data']['uid'];
```
