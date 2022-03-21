---
title: Convoy-PHP SDK
description: 'Convoy-PHP SDK Configuration'
id: convoy-php
order: 8
---


#### Installation

To install the package, you will need to be using Composer in your project.

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
```
