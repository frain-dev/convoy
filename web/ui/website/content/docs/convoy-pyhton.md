---
title: Convoy-Python SDK
description: 'Convoy-Python SDK Configuration'
id: convoy-python
order: 7
---


#### Installation
Install convoy-python with

```bash
pip install convoy-python
```

#### Setup Client
Next, import the `convoy` module and setup with your auth credentials.

```python
from convoy import Convoy
convoy = Convoy({"api_key":"your_api_key"})
```
The SDK also supports authenticating via Basic Auth by defining your username and password.

```python
convoy = Convoy({"username":"default", "password":"default"})
```

In the event you're using a self hosted convoy instance, you can define the url as part of what is passed into convoy's constructor.

```python
convoy = Convoy({ "api_key": 'your_api_key', "uri": 'self-hosted-instance' })
```

