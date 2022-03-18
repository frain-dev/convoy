---
title: SDK
description: 'Convoy SDK Configuration'
id: sdk
order: 5
---

# SDK

Convoy SDKs are available for Javascript, PHP and Python.

## Convoy.js

#### Installation
Install convoy.js with

```bash
$ npm install convoy.js
```

#### Setup Client

Next, require the `convoy` module and setup with your auth credentials.

```json[]
    const { Convoy } = require('convoy.js');
    const convoy = new Convoy({ api_key: 'your_api_key' })
```

The SDK also supports authenticating via Basic Auth by defining your username and password.

```json[]
    const convoy = new Convoy({ username: 'default', password: 'default' })
```

In the event you're using a self hosted convoy instance, you can define the url as part of what is passed into convoy's constructor.
```json[]
    const convoy = new Convoy({ api_key: 'your_api_key', uri: 'self-hosted-instance' })
```

#### Usage

```json[]  
//async await
try {
    await convoy.groups.all({ perPage: 10, page: 1 })
} catch(error) {
    console.log(error)
}

//promises
convoy.groups
.all()
.then(res => console.log(res))
.catch(err => console.log(err))
```

#### Testing

```bash
$ npm run test
```

## Convoy-Python
Convoy SDK for Python

This is the Convoy Python SDK. This SDK contains methods for easily interacting with Convoy's API. Below are examples to get you started. For additional examples, please see our official documentation at (https://convoy.readme.io/reference)


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

#### Usage

```python
content, status = convoy.group.all({ "perPage": 10, "page": 1 })
```

#### Testing

```python
pytest ./test/test.py
```


## Convoy SDK for PHP


This is the Convoy PHP SDK. This SDK contains methods for easily interacting with Convoy's API. Below are examples to get you started. For additional examples, please see our official documentation at (https://convoy.readme.io/reference)


#### Installation
To install the package, you will need to be using Composer in your project. 

The Convoy PHP SDK is not hard coupled to any HTTP Client such as Guzzle or any other library used to make HTTP requests. The HTTP Client implementation is based on [PSR-18](https://www.php-fig.org/psr/psr-18/). This provides you with the convenience of choosing what [PSR-7](https://packagist.org/providers/psr/http-message-implementation) and [HTTP Client](https://packagist.org/providers/psr/http-client-implementation) you want to use.

To get started quickly, 

```bash
composer require frain/convoy symfony/http-client nyholm/psr7
```

#### Usage

```php
use Convoy\Convoy;


$config = [
    'api_key' => 'your_api_key',
    'uri' => 'https://self-hosted-convoy' //This is optional and will default to https://cloud.getconvoy.io/api/v1
];

$convoy = new Convoy($config);

//Group Resource
$groups = $convoy->groups()->all();
$group = $convoy->groups()->find('group-uuid')
```

#### Testing

```bash
composer test
```
