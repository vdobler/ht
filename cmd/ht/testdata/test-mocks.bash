#! /bin/bash
#
# test-mocks.bash test the mock subcommand of cmd/ht
#

set -euo pipefail

# Start of mock service
../ht mock -D POST="Ciao" -cert ../../../mock/testdata/dummy.cert -key ../../../mock/testdata/dummy.key ../../../mock/testdata/body.mock &
pid=$!
sleep 2
kill -0 $pid || (echo "Problems running mock server."; echo "FAIL"; exit 1;)

function finish {
    rm -f alf.de alf.en headmaster.de headmaster.en indiana.de indiana.en combined expected
    kill -TERM $pid
}
trap finish EXIT

curl -# -k -d first="Gordon" -d last="Shumway" https://localhost:8880/de/greet > alf.de
curl -# -k -d first="Gordon" -d last="Shumway" https://localhost:8880/en/greet > alf.en
curl -# -k -d first="Henry Walton" -d last="Jones" https://localhost:8880/de/greet > indiana.de
curl -# -k -d first="Henry Walton" -d last="Jones" https://localhost:8880/en/greet > indiana.en
curl -# -k -d first="Albus" -d last="Dumbledore" https://localhost:8880/de/greet > headmaster.de
curl -# -k -d first="Albus" -d last="Dumbledore" https://localhost:8880/en/greet > headmaster.en


cat alf.de alf.en indiana.de indiana.en headmaster.de headmaster.en > combined

cat > expected <<EOF
Guten Tag Gordon Shumway
Ciao
Welcome Gordon Shumway
Ciao
Guten Tag Dr. Henry Walton 'Indiana' Jones
Ciao
Welcome Henry Walton Jones
Ciao
Guten Tag Prof. Albus Dumbledore
Ciao
Welcome Albus Dumbledore
Ciao
EOF

if ! diff -u combined expected ; then
    echo "Wrong output"
    echo "FAIL"
    exit 1
fi
echo "PASS"

