#!/bin/sh
exec redis-server --requirepass "$(cat /run/secrets/redis_password)"
