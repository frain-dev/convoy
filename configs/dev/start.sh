#!/bin/sh

./cmd migrate up
./cmd server --config convoy.json
