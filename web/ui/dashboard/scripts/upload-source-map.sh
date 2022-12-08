NODE_ENV="${NODE_ENV}"
PRODUCTION_ENV="production"

if [ "${NODE_ENV}" == "${PRODUCTION_ENV}" ]; then
    npm run sentry:release
fi
