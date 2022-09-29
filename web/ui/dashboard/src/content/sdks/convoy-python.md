### Installation
Install convoy-python with

```bash[terminal]
$ pip install convoy-python
```

### Setup Client
Next, import the `convoy` module and setup with your auth credentials.

```python[example]
from convoy import Convoy
convoy = Convoy({"api_key":"your_api_key"})
```
The SDK also supports authenticating via Basic Auth by defining your username and password.

```python[example]
convoy = Convoy({"username":"default", "password":"default"})
```

In the event you're using a self hosted convoy instance, you can define the url as part of what is passed into convoy's constructor.

```python[example]
convoy = Convoy({ "api_key": 'your_api_key', "uri": 'self-hosted-instance' })
```
### Creating an Application

An application represents a user's application trying to receive webhooks. Once you create an application, you'll receive a `uid` that you should save and supply in subsequent API calls to perform other requests such as creating an event.

```python[example]
appData = { "name": "my_app", "support_email": "support@myapp.com" }
(response, status)  = convoy.applications.create({}, appData)
appId = response["data"]["uid"]

```

### Add Application Endpoint

After creating an application, you'll need to add an endpoint to the application you just created. An endpoint represents a target URL to receive events.

```python[example]
endpointData = {
    "url": "https://0d87-102-89-2-172.ngrok.io",
    "description": "Default Endpoint",
    "secret": "endpoint-secret",
    "events": ["*"],
  }

(response, status) = convoy.endpoint.create(appId, {}, endpointData)
```

### Sending an Event

To send an event, you'll need the `uid` we created in the earlier section.

```python[example]
eventData = {
    "app_id": appId,
    "event_type": "payment.success",
    "data": {
      "event": "payment.success",
      "data": {
        "status": "Completed",
        "description": "Transaction Successful",
        "userID": "test_user_id808",
      },
    },
  }

(response, status) = convoy.event.create({}, eventData)
```
