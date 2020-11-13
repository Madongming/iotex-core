#!/bin/bash

IOTEX_HOME=$(cat ${HOME}/.iotex/workdir)
PROJECT_ABS_DIR=$(cd "$(dirname "$0")";pwd)

function main() {
    \cp -r $PROJECT_ABS_DIR/example $IOTEX_HOME/
    python -m webbrowser http://ide.iotex.io/
    ioctl contract share $IOTEX_HOME/example/contract
}

main $@
