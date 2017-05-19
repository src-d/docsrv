#!/bin/sh

# clone shared repo if any
if [ -n $SHARED_REPO ]; then
        git clone $SHARED_REPO /etc/shared
        if [ $? -ne 0 ]; then
                exit 1;
        fi
fi

# create 404 page if not exists
if [ ! -f /var/www/public/errors/404.html ]; then
        echo '<h1>Not Found</h1>' > /var/www/public/errors/404.html
fi

# create 500 page if not exists
if [ ! -f /var/www/public/errors/500.html ]; then
        echo '<h1>Server Error</h1>' > /var/www/public/errors/500.html
fi

/usr/bin/supervisord
