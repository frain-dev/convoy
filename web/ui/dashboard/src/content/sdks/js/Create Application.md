### Creating an Application

An application represents a user's application trying to receive webhooks. Once you create an application, you'll receive a `uid` as part of the response that you should save and supply in subsequent API calls to perform other requests such as creating an event.

```js[example]
try {
  const appData = { name: "my_app", support_email: "support@myapp.com" };

  const response = await convoy.application.create(appData);

  const appId = response.data.uid;
} catch (error) {
  console.log(error);
}
```
