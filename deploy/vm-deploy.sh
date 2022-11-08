#!/bin/sh

set -e 

VERSION="latest"
DOMAIN="localhost"
CONFDIR="/etc/convoy"
COMPOSECONFDIR="$CONFDIR/compose"
DATADIR="/var/convoy/data"

# Read Convoy version
read_version() {
	echo
	echo "What version of Convoy would you like to install? (We default to the latest)"
	echo "You can check out available versions here: https://github.com/frain-dev/convoy/releases"

	local version=""

	read -p "Version: " version

	if [ -z "$version" ]
	then
		echo "Using default and installing $VERSION"
	else
		export VERSION=$version
		echo "Using provided version $VERSION"
	fi
}

# Ask if we should generate tls or not?
should_setup_tls() {
	echo
	while true; do
		echo "Should we setup a TLS certificate for you using Let's Encrypt?"
		echo "Select no if you are using this internally and Convoy will not be reachable from the intenet."
		read -p "TLS (y/n): " yn
		case $yn in 
			[Yy]* ) export USE_SELF_SIGNED_CERT=0; break ;;
			[Nn]* ) export USE_SELF_SIGNED_CERT=1; break ;;
			* ) echo "Please answer yes or no." ;;
		esac
	done
}

# Get domain from user.
get_domain_name() {
	echo
	read -p "Domain: " DOMAIN
	export DOMAIN=$DOMAIN
	echo "We will set up certs for https://$DOMAIN"
}

# Grant installer sudo access
get_sudo_access() {
	echo 
	echo "We will need sudo access so the next question is for you to give us superuser access"
	echo "Please enter your sudo password now:"
	sudo echo ""
}

# Stop any running Convoy cluster.
stop_containers() {
	sudo -E docker-compose -f docker-compose.yml stop &> /dev/null || true
}

# Installer grabs all necessary dependencies.
get_dependencies() {
	echo
	echo "Grabbing latest apt caches"
	sudo apt install -y apt-transport-https ca-certificates curl software-properties-common
	curl -fsSL https://download.docker.com/linux/ubuntu/gpg | sudo -E apt-key add -
	sudo add-apt-repository "deb [arch=amd64] https://download.docker.com/linux/ubuntu bionic stable"
	sudo apt update
	sudo apt-cache policy docker-ce
	sudo apt install -y docker-ce	git jq

	# setup docker-compose
	echo "Setting up Docker Compose"
	sudo curl -L "https://github.com/docker/compose/releases/download/1.27.4/docker-compose-$(uname -s)-$(uname -m)" \
					 	-o /usr/local/bin/docker-compose || true
	sudo chmod +x /usr/local/bin/docker-compose

	# enable docker without sudo
	sudo usermod -aG docker "${USER}"
}

# clone convoy repository
install_convoy() {
	echo 
	echo "Installing Convoy from Github"
	git clone https://github.com/frain-dev/convoy.git &> /dev/null || true
	cd convoy
	git pull
	cd ..
}

# This enables this script to be backward compatible with previous scripts.
prepare_directories() {
	echo 
	echo "Preparing configuration directories ..."

	mkdir -p $CONFDIR
	mkdir -p $DATADIR
	mkdir -p $COMPOSECONFDIR

}

copy_configurations() {
	echo 

	cp convoy/configs/docker-compose.templ.yml $CONFDIR
	cp convoy/configs/convoy.templ.json $CONFDIR
	cp convoy/configs/caddyfile $CONFDIR

	cd $CONFDIR
}

# rewrite convoy.json, caddyfile & docker-compose
write_configurations() {
	# rewrite caddyfile
	rm -f caddyfile
envsubst > caddyfile <<EOF
$DOMAIN, :80, :443 {
$TLS_BLOCK
reverse_proxy http://web:5005
}
EOF

	# rewrite convoy.json
	echo "Setting up convoy.json ..."
	echo "$( jq --arg domain "${DOMAIN}" '.host = $domain | .environment = "production"' convoy.templ.json  )" > convoy.json
	rm convoy.templ.json
	echo "convoy.json ready"
	echo

	# rewrite docker compose
	envsubst < docker-compose.templ.yml > docker-compose.yml
	rm docker-compose.templ.yml

	# Fix compose start command
	cat > $COMPOSECONFDIR/start <<EOF
./cmd migrate up
./cmd server --config convoy.json -w false
EOF
}

# setup replica set on mongo db clusters
setup_replica_set() {
	echo
	docker exec mongo1 mongosh --eval "rs.initiate({
   _id: \"myReplicaSet\",
   members: [
     {_id: 0, host: \"mongo1\"},
     {_id: 1, host: \"mongo2\"},
     {_id: 2, host: \"mongo3\"}
   ]
	})"
}

# start system
start_containers() {
	echo
	echo "Starting containers..."
	sudo -E docker-compose -f docker-compose.yml up -d
}

# check if the server is ready to start receiving requests
check_if_containers_are_up() {
	echo
	echo "We will need to wait ~5-10 minutes for things to settle down and TLS certs to be issued"
	echo 
	echo "⏳ Waiting for Convoy web to boot (this will take a few minutes)"
	bash -c 'while [[ "$(curl -s -o /dev/null -w ''%{http_code}'' localhost:5005/health)" != "200" ]]; do sleep 5; done'
	echo "⌛️ Convoy looks up!"
	echo 
	echo "🎉🎉🎉  Done! 🎉🎉🎉"
	echo 
	echo "To stop the stack run 'docker-compose stop'"
	echo "To start the stack again run 'docker-compose start'"
	echo "If you have any issues at all delete everything in this directory and run the curl command again"
	echo 
	echo "Convoy will be up at the location you provided!"
	echo "https://${DOMAIN}"
	echo 
}

start() {
	echo "Welcome to the single instance Convoy installer"
	echo 
	echo "⚠️  You really need 4gb or more of memory to run this stack ⚠️  "
	
	# Ask for version
	read_version

	# Collect domain name
	get_domain_name

	# Should we setup TLS?
	should_setup_tls

	# Update apt caches
	get_dependencies

	# Install Convoy
	install_convoy

	# preprare directories
	prepare_directories

	# copy configurations
	copy_configurations

	# setup configuration 
	write_configurations

	# stop previously running containers
	stop_containers

	# start containers
	start_containers

	# setup replica set
	setup_replica_set

	# check if services are up
	check_if_containers_are_up
}

start
