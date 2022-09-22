#!/bin/bash

# https://linuxtect.com/make-bash-shell-safe-with-set-euxo-pipefail/
# https://gist.github.com/vncsna/64825d5609c146e80de8b1fd623011ca
set -euo pipefail


HOST=$1
ENDPOINT=$2
BASE_URL=$HOST/api/v1

have_jq=$(which jq)

# create the test group
curl --location --request POST $BASE_URL/groups \
--header 'Content-Type: application/json' \
--data-raw '{
    "config": {
        "disableEndpoint": false,
        "signature": {
            "hash": "SHA256",
            "header": "X-Test-Signature"
        },
        "strategy": {
            "default": {
                "intervalSeconds": 60,
                "retryLimit": 10
            },
            "type": "default"
        }
    },
    "rate_limit": 5000,
    "rate_limit_duration": "1m",
    "name": "load-test-group"
}'

# get groups
groups=$(curl --location --request GET "$BASE_URL/groups")

# get group for test
groupId=$(echo $groups | jq --raw-output .data[0].uid)

curl --location --request POST "$BASE_URL/applications?groupId=$groupId" \
--header 'Content-Type: application/json' \
--data-raw '{
    "name": "staging",
    "support_email": "getconvoy@gmail.com"
}'

# get apps
apps=$(curl --location --request GET "$BASE_URL/applications?groupId=$groupId")

# get group for test
appId=$(echo $apps | jq --raw-output .data.content[-1:][0].uid)

curl --location --request POST "$BASE_URL/applications/$appId/endpoints?groupId=$groupId" \
--header 'Content-Type: application/json' \
--data-raw "{ \"url\": \"$ENDPOINT\", \"description\": \"John wants to wick\", \"secret\": \"john-wicks-dog\" }"

echo "group id: $groupId, app id $appId"
