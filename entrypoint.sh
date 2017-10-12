#!/bin/sh

# run init scripts, if any
echo RUNNING SCRIPTS
ls -la /etc/docsrv/init.d
for file in /etc/docsrv/init.d/*.sh; do
	echo first is $file
	if [ -f $file ]; then
			sh $file;
	fi
done

# create 404 page if not exists
if [ ! -f /var/www/public/errors/404/index.html ]; then
	echo TRYING TO CREATE 404
	mkdir -p /var/www/public/errors/404
	echo '<h1>Not Found</h1>' > /var/www/public/errors/404/index.html;
fi

# create 500 page if not exists
if [ ! -f /var/www/public/errors/500/index.html ]; then
	echo TRYING TO CREATE 500
	mkdir -p /var/www/public/errors/500
	echo '<h1>Server Error</h1>' > /var/www/public/errors/500/index.html;
fi

/usr/bin/supervisord
