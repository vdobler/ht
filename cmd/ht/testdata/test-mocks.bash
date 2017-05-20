#! /bin/bash
#
# test-mocks.bash test the mock subcommand of cmd/ht
#

# Start of mock service

../ht mock -D POST="good buy" -cert ../../../mock/testdata/dummy.cert -key ../../../mock/testdata/dummy.key ../../../mock/testdata/body.mock &

pid=$!

sleep 1

for lastname in Alf Jones Dumbledore; do
    curl -k -d last=$lastname -d first=Mike https://localhost:8880/de/greet

    echo
done





kill -TERM $pid
