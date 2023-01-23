#!/bin/sh
cat ./uffizzi/convoy-uffizzi.json > convoy.json
./cmd migrate up
./cmd server --config convoy.json
