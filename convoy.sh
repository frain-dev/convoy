#!/bin/sh

exec_assert()
{
    $@
    if [ $? -ne 0 ]
    then
      echo "$@ failed"; exit 1
    fi
}

build_react_files()
{
  INSTALL_NODE_VER=14

  cd $SCRIPT_DIR/web/ui/dashboard
  echo $SCRIPT_DIR

  source ~/.nvm/nvm.sh
  if ! command -v nvm &> /dev/null
  then
      echo "nvm not installed"; exit 1
  fi

  echo "==> Installing node js version $INSTALL_NODE_VER"
  nvm install $INSTALL_NODE_VER

  exec_assert "npm install"
  exec_assert "npm run build"
  exec_assert "cp -r dist/* $SCRIPT_DIR/server/ui/build"
  exec_assert "rm -rf dist"
}

SCRIPT_DIR="$( cd "$( dirname "${BASH_SOURCE[0]}" )" &> /dev/null && pwd )"

IS_SERVER=false
IS_HELP=false
for i in "$@" ; do
    if [[ $i == "server" || $i == "s" ]] ; then
        IS_SERVER=true
        continue
    fi
    if [[ $i == "--help" || $i == "-h" ]] ; then
        IS_HELP=true
    fi
done

if [[ $IS_SERVER == true && $IS_HELP == false ]]; then
    build_react_files
fi

cd $SCRIPT_DIR/cmd

go run *.go $@
