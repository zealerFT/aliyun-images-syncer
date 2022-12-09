FROM alpine

MAINTAINER fermi <spuerfei@gmail.com>

LABEL name="images-sync" \
description="Master slave image automatic synchronization service by fermi" \
owner="spuerfei@gmail.com"

WORKDIR /bin

ADD bin/fermi /bin/fermi
COPY docker/entrypoint.sh /entrypoint.sh

ENTRYPOINT ["/entrypoint.sh"]
