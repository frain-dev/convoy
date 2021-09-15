#!/bin/sh

export CONVOY_API_USERNAME=apiUsername
export CONVOY_API_PASSWORD=apiPassword
export CONVOY_UI_USERNAME=uiUsername
export CONVOY_UI_PASSWORD=uiPassword
export CONVOY_JWT_EXPIRY=3600
export CONVOY_JWT_KEY=subomiOluwalana

docker-compose up
