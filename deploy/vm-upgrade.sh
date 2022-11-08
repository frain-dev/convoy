#!/bin/sh

set -e 

CONFDIR="/etc/convoy"
COMPOSECONFDIR="$CONFDIR/compose"

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

export VERSION="${VERSION:-latest}"

cd $CONFDIR

install_convoy

rm -f docker-compose.yml
cp convoy/configs/docker-compose.templ.yml $CONFDIR


envsubst < docker-compose.templ.yml > docker-compose.yml
rm docker-compose.templ.yml

# backward compatible fixes
[[ -f "Caddyfile" ]] && mv Caddyfile caddyfile
[[ -d "/var/convoy/data/mongo1" ]] && mv /var/convoy/data/mongo1 ./mongo1-data
[[ -d "/var/convoy/data/mongo2" ]] && mv /var/convoy/data/mongo2 ./mongo2-data
[[ -d "/var/convoy/data/mongo3" ]] && mv /var/convoy/data/mongo3 ./mongo3-data
[[ -d "/var/convoy/data/typesense" ]] && mv /var/convoy/data/typesense ./typesense-data


docker-compose pull

echo "Stopping the system!"
docker-compose stop

# Fix compose start command
cat > $COMPOSECONFDIR/start <<EOF
#!/bin/bash
./cmd migrate up
./cmd server --config convoy.json -w false
EOF
chmod +x $COMPOSECONFDIR/start

echo "Restarting the system!"
sudo -E docker-compose up -d

# setup replica set.
docker exec mongo1 mongosh --eval "rs.initiate({
   _id: \"myReplicaSet\",
   members: [
     {_id: 0, host: \"mongo1\"},
     {_id: 1, host: \"mongo2\"},
     {_id: 2, host: \"mongo3\"}
   ]
})"

echo "Convoy upgraded successfully"
