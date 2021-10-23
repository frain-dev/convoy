---
title: Getting started
description: 'Easy started with Convoy'
id: welcome
---

# Quick start guide

## 1. Create Directory
```bash[bash]
$ mkdir convoy && cd convoy
```

## 2. Copy Configuration 
Copy both the compose file and the configuration file to the directory created above.

```yml[docker-compose.yml]
version: "3"

services:
    web:
        image: ghcr.io/frain-dev/convoy:0.2.4
        entrypoint: ["./cmd", "server", "--config", "convoy.json"]
        ports:
            - 5005:5005
        volumes:
            - ./convoy.json:/convoy.json
        restart: on-failure
        depends_on:
            - mongodb
            - redis_server

    mongodb:
        image: mongo:latest
        environment:
            MONGO_INITDB_ROOT_USERNAME: root
            MONGO_INITDB_ROOT_PASSWORD: rootpassword
        volumes:
            - ./data/mongo:/data/db
        ports:
           - "27017:27017"

    redis_server:
        image: redis:alpine
        ports:
            - "8379:6379"
```

```json[convoy.json]
{
  "database": {
    "dsn": "mongodb://root:rootpassword@mongodb:27017"
  },
  "queue": {
    "type": "redis",
    "redis": {
      "dsn": "redis://redis_server:6379"
    }
  },
  "server": {
    "http": {
      "port": 5005
    }
  },
  "auth": {
    "type": "none"
  },
  "strategy": {
    "type": "default",
    "default": {
      "intervalSeconds": 120,
      "retryLimit": 10
    }
  }
}
```

## 3. Start Containers
```bash[bash]
$ docker-compose up
```
Now, you can head over to http://localhost:5005 to view the UI, which should look something like:
![convoy image](../../docs-assets/convoy-ui.png)

You can check the [config page](./docs/configuration) for full details on configuration.

## 4. Send Webhooks
Now, let's send webhooks. We maintain an openapi spec in the repository, you can download it [here](https://raw.githubusercontent.com/frain-dev/convoy/main/docs/v3/openapi3.json) and import to Insomnia or Postman to get started.

### 4.1 Create Application
```json[Sample Payload]
{
    "name": "myapp",
    "secret": "eyJhbGciOiJIUzI1NiJ9",
    "support_email": "support@myapp.com"
}
```
```bash[bash]
$ curl \
    --request POST \
    --data @app.json \
    -H "Content-Type: application/json" \
    http://localhost:5005/api/v1/applications
```

```json[Response]
{
    "status":true,
    "message":"App created successfully",
    "data":{
        "uid":"e0e1240a-96dc-4408-a335-144eb3749d34",
        "group_id":"1e6c46cf-9c8a-4ec2-85f7-69d3d2009b94",
        "name":"myapp",
        "support_email":"support@myapp.com",
        "secret":"eyJhbGciOiJIUzI1NiJ9",
        "endpoints":[],
        "created_at":"2021-10-23T13:37:13.642Z",
        "updated_at":"2021-10-23T13:37:13.642Z",
        "events":0
    }
}
```

### 4.2 Add Endpoint
```json[Sample Payload]
{
    "description": "Default Endpoint",
    "url": "https://0d87-102-89-2-172.ngrok.io"
}
```

```bash[bash]
$ curl \
    --request POST \
    --data @endpoint.json \
    -H "Content-Type: application/json" \
    http://localhost:5005/api/v1/applications/e0e1240a-96dc-4408-a335-144eb3749d34/endpoints
```

```json[Response]
{
    "status":true,
    "message":"App endpoint created successfully",
    "data":{
        "uid":"343110ab-8800-47ad-9452-e6df2e14746c",
        "target_url":"https://0d87-102-89-2-172.ngrok.io",
        "description":"Default Endpoint",
        "status":"active",
        "created_at":"2021-10-23T13:43:36.937Z",
        "updated_at":"2021-10-23T13:43:36.937Z"
    }
}
```

### 4.3 Send Event
```json[Sample Payload]
{
	"app_id": "e0e1240a-96dc-4408-a335-144eb3749d34",
	"event_type": "payment.success",
	"data": {
		"event": "payment.success",
		"data": {
			"status": "Completed",
			"description": "Transaction Successful",
			"userID": "test_user_id808",
			"paymentReference": "test_ref_85149",
			"amount": 200,
			"senderAccountName": "Alan Christian Segun",
			"sourceAccountNumber": "2999993564",
			"sourceAccountType": "personal",
			"sourceBankCode": "50211",
			"destinationAccountNumber": "00855584818",
			"destinationBankCode": "063"
		}
	}
}
```

```bash[bash]
$ curl \
    --request POST \
    --data @event.json \
    -H "Content-Type: application/json" \
    http://localhost:5005/api/v1/events
```

```json[Response]
{
    "status":true,
    "message":"App event created successfully",
    "data":{
        "uid":"d740e1eb-37c6-42de-a8ef-b4821e8bae2b",
        "app_id":"e0e1240a-96dc-4408-a335-144eb3749d34",
        "event_type":"payment.success",
        "provider_id":"e0e1240a-96dc-4408-a335-144eb3749d34",
        "data":{
            "event":"payment.success",
            "data":{
                "status":"Completed",
                "description":"Transaction Successful",
                "userID":"test_user_id808",
                "paymentReference":"test_ref_85149",
                "amount":200,
                "senderAccountName":"Alan Christian Segun",
                "sourceAccountNumber":"2999993564",
                "sourceAccountType":"personal",
                "sourceBankCode":"50211",
                "destinationAccountNumber":"00855584818",
                "destinationBankCode":"063"
            }
        },
        "metadata":{
            "strategy":"default",
            "next_send_time":"2021-10-23T14:09:31.839Z",
            "num_trials":0,
            "interval_seconds":20,
            "retry_limit":3
        },
        "status":"Scheduled",
        "app_metadata":{
            "group_id":"1e6c46cf-9c8a-4ec2-85f7-69d3d2009b94",
            "secret":"eyJhbGciOiJIUzI1NiJ9",
            "support_email":"support@myapp.com",
            "endpoints":[
                {
                    "uid":"343110ab-8800-47ad-9452-e6df2e14746c",
                    "target_url":"https://0d87-102-89-2-172.ngrok.io",
                    "status":"",
                    "sent":false
                }
            ]
        },
        "created_at":"2021-10-23T14:09:31.839Z",
        "updated_at":"2021-10-23T14:09:31.839Z"
    }
}
```

## 5. Receive Webhooks
Let's write a basic ruby app to receive events.
```ruby
require 'sinatra'
require 'openssl'

post '/' do
    secret = "eyJhbGciOiJIUzI1NiJ9"
    body = request.body.read

    hook_signature = request.env['HTTP_X_CONVOY_PLAYGROUND_SIGNATURE']
    digest = OpenSSL::Digest::SHA512.new
    signature = OpenSSL::HMAC.hexdigest(digest, secret, body)

    is_valid = Rack::Utils.secure_compare(signature, hook_signature)

    if is_valid
        status 200
        body 'Got it, thanks Convoy!'
    else
        puts "It didn't"
    end
end

```

The UI should look like this at this point.
![convoy full image](../../docs-assets/convoy-full-ui.png)

And that's it!
