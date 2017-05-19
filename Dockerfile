FROM abiosoft/caddy

RUN apk update \
        && apk add --no-cache supervisor \
        && apk add --no-cache git \
        && apk add --no-cache build-base

COPY entrypoint.sh .
COPY supervisord.conf /etc/supervisord.conf
COPY Caddyfile /etc/Caddyfile
COPY bin /bin
RUN mkdir -p /etc/shared

ENTRYPOINT ["./entrypoint.sh"]
