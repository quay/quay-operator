#!/bin/bash

errs=$(go fmt ./...)
if [ "$errs" != "" ]; then
	echo "format errors"
	git diff
	exit 1
fi
