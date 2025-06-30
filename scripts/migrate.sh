#!/bin/sh

DSN="postgresql://root@lb:26257/authserviceprod?sslcert=./cockroach/certs/client.root.crt&sslkey=./cockroach/certs/client.root.key&sslrootcert=./cockroach/certs/ca.crt&sslmode=verify-full"
cockroach sql --certs-dir=./cockroach/certs --host=lb --database=authserviceprod < migrations/*.up.sql