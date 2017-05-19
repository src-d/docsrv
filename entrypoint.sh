#!/bin/sh

# clone shared repo if any
if [ -n $SHARED_REPO ]; then
        git clone $SHARED_REPO /etc/shared
        if [ $? -ne 0 ]; then
                exit 1;
        fi
fi

/usr/bin/supervisord
