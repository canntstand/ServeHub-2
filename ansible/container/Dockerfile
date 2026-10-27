FROM alpine/ansible:2.21.0

# The upstream tag can lag security updates in Alpine repositories.
RUN apk upgrade --no-cache \
    && apk add --no-cache py3-passlib py3-bcrypt
