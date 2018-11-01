# FROM postgres:10.2-alpine
FROM mdillon/postgis:10-alpine

COPY init /docker-entrypoint-initdb.d
COPY postgres-entrypoint.sh /docker-entrypoint.sh

VOLUME /var/lib/postgresql/data
