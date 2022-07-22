#!/bin/sh

set -e 

CONFDIR="/etc/convoy"

# re-install convoy, again for backward compatibility
install_convoy() {
	echo 
	echo "Installing Convoy from Github"
	git clone https://github.com/frain-dev/convoy.git &> /dev/null || true
	cd convoy
	git pull
	cd ../
}

echo "Upgrading Convoy. This will cause a few minutes of downtime."
read -r -p "Do you want to upgarde Convoy? [y/N] " response
if [[ "$response" =~ ^([yY][eE][sS]|[yY])+$ ]]
then
	echo "OK!"
else
	exit
fi

[[ -f "convoy.json" ]] || ( echo "No convoy.json file found. Please ensure you're in the right directory" && exit 1)
export VERSION="${VERSION:-latest}"

cd $CONFDIR

install_convoy

rm -f docker-compose.yml
cp convoy/configs/docker-compose.templ.yml $CONFDIR


envsubst < docker-compose.templ.yml > docker-compose.yml
rm docker-compose.templ.yml

docker-compose pull

echo "Stopping the system!"
docker-compose stop

echo "Restarting the system!"
sudo -E docker-compose up -d

echo "Convoy upgraded successfully"
