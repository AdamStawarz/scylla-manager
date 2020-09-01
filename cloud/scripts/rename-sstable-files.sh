#!/bin/bash
#
# Copyright (C) 2017 ScyllaDB
#

set -eu -o pipefail

if [[ $# -eq 0 ]]; then
    echo "No arguments supplied"
    exit 1
fi

if [[ $1 == "" ]]; then
    echo "Empty argument supplied"
    exit 1
fi

for f in $(find "$1" -regextype posix-extended -regex '.*/[a-zA-Z]+-([0-9]+)-[a-zA-Z]+-[a-zA-Z]+\..*$' -type f)
do
    mv "$f" $(sed -rn 's|-([[:digit:]]+)-|-\10-|p' <<< "$f")
done