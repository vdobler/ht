#! /bin/bash
#
# test-error.bash tests some error conditions
#

set -euo pipefail

# ----------------------------------------------------------------------------
#

cat > buggy.suite <<EOF
{
    Name: "Make three arbitrary GET request"
    Setup:    {File: "a.ht"}
    Main:     {File: "b.ht"}
    Teardown: {File: "c.ht"}
}
EOF

cat > x.ht <<EOF
{
    Name: "Request",
    Request: { URL: "http://www.google.com" }
}
EOF



echo
echo "General Suite Execution"
echo "======================="
cp x.ht a.ht
cp x.ht b.ht
cp x.ht c.ht

rm -rf okay
./ht exec -ss -output okay buggy.suite || \
    (echo "FAIL: exec buggy.suite $?"; exit 1;)
rm -f a.ht b.ht c.ht

echo
echo "Missing Suite"
echo "============="
rm -rf error
./ht exec -ss -output error buggyXXX.suite && \
    (echo "FAIL: exec buggyXXX.suite $?"; exit 1;)
rm -f a.ht b.ht c.ht

echo
echo "Missing Setup"
echo "============="
rm -f   a.ht
cp x.ht b.ht
cp x.ht c.ht

rm -rf error
./ht exec -ss -output error buggy.suite && \
    (echo "FAIL: exec buggy.suite $?"; exit 1;)
rm -f a.ht b.ht c.ht

echo
echo "Missing Main"
echo "============"
cp x.ht a.ht
rm -f   b.ht
cp x.ht c.ht

rm -rf error
./ht exec -ss -output error buggy.suite && \
    (echo "FAIL: exec buggy.suite $?"; exit 1;)
rm -f a.ht b.ht c.ht

echo
echo "Missing Teardown"
echo "================"
cp x.ht a.ht
cp x.ht b.ht
rm -f   c.ht

rm -rf error
./ht exec -ss -output error buggy.suite && \
    (echo "FAIL: exec buggy.suite $?"; exit 1;)
rm -f a.ht b.ht c.ht

echo
echo "Missing Mixin"
echo "============="
cp x.ht a.ht
cp x.ht c.ht
cat > b.ht <<EOF
{
    Mixin: [ "m.mix" ]
    Name: "Request",
    Request: { URL: "http://www.google.com" }
}
EOF

rm -rf error
./ht exec -ss -output error buggy.suite && \
    (echo "FAIL: exec buggy.suite $?"; exit 1;)
rm -f a.ht b.ht c.ht

echo
echo "Missing Mock"
echo "============="
cp x.ht a.ht
cat > buggy.suite <<EOF
{
    Name: "Make three GET"
    Main:     {File: "a.ht", Mocks: [ "m.mock"]}
}
EOF

rm -rf error
./ht exec -ss -output error buggy.suite && \
    (echo "FAIL: exec buggy.suite $?"; exit 1;)
rm -f a.ht b.ht c.ht


rm -rf buggy.suite x.ht a.ht b.ht c.ht okay error

echo "PASS"
exit 0
