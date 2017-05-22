FROM abiosoft/caddy

RUN apk update \
        && apk add --no-cache supervisor \
        && apk add --no-cache git \
        && apk add --no-cache build-base

RUN mkdir -p /etc/shared \
        && mkdir -p /var/www/public/errors
COPY entrypoint.sh .
COPY supervisord.conf /etc/supervisord.conf
COPY bin /bin
COPY Caddyfile /etc/Caddyfile

ENTRYPOINT ["./entrypoint.sh"]
