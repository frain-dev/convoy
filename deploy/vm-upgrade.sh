#!/bin/sh

set -e 

CONFDIR="/etc/convoy"

echo "Upgrading Convoy. This will cause a few minutes of downtime."
read -r -p "Do you want to upgarde Convoy? [y/N] " response
if [[ "$response" =~ ^([yY][eE][sS]|[yY])+$ ]]
then
	echo "OK!"
else
	exit
fi

[[ -f "convoy.json" ]] && export $(cat convoy.json | xargs) || ( echo "No convoy.json file found. Please ensure you're in the right directory" && exit 1)
export VERSION="${VERSION:-latest}"

cd convoy
git pull
cd ../

rm -f docker-compose.yml
cp convoy/configs/docker-compose.templ.yml $CONFDIR

cd $CONFDIR

envsubst < docker-compose.templ.yml > docker-compose.yml
rm docker-compose.templ.yml

docker-compose pull

echo "Stopping the system!"
docker-compose stop

echo "Restarting the system!"
sudo -E docker-compose up -d

echo "Convoy upgraded successfully"
