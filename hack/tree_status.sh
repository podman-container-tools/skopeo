#!/usr/bin/env bash
set -e

STATUS=$(git status --porcelain)
if [[ -z $STATUS ]]
then
	echo "tree is clean"
else
	echo "tree is dirty, please commit all changes"
	echo ""
	echo "$STATUS"
	exit 1
fi
