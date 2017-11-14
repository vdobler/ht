#! /bin/bash
#
# test-exitcode.bash tests exit code of ht run and ht exec.
#

set -u

# ----------------------------------------------------------------------------
# Suite1: general variable handling
#

cat > bogus.ht <<EOF
{
    Name: "Bogus"
    Request: { URL: "tamdidadum" }
    Checks: [ {Check: "StatusCode", Expect: 200}  ]
}
EOF

cat > error.ht <<EOF
{
    Name: "Error"
    Request: { URL: "http://example.org", Timeout: "1ms" }
    Checks: [ {Check: "StatusCode", Expect: 200}  ]
}
EOF

cat > fail.ht <<EOF
{
    Name: "Fail"
    Request: { URL: "http://example.org" }
    Checks: [ {Check: "StatusCode", Expect: 789}  ]
}
EOF


cat > pass.ht <<EOF
{
    Name: "Pass"
    Request: { URL: "http://example.org" }
    Checks: [ {Check: "StatusCode", Expect: 200}  ]
}
EOF

cat > exit.suite <<EOF
{
    Name: "Exit Status Suite"
    Main: [
        {File: "pass.ht"}
        {File: "fail.ht"}
        {File: "error.ht"}
        {File: "bogus.ht"}
    ]
}
EOF



echo
echo "Exit Code of 'ht run'"
echo "====================="

./ht run -ss pass.ht
ec=$?
[[ $ec == 0 ]]  || { echo "FAIL: pass.ht got exit code of $ec, want 0"; exit 1; }

./ht run -ss fail.ht
ec=$?
[[ $ec == 1 ]]  || { echo "FAIL: fail.ht got exit code of $ec, want 1"; exit 1; }

./ht run -ss error.ht
ec=$?
[[ $ec == 2 ]]  || { echo "FAIL: error.ht got exit code of $ec, want 2"; exit 1; }

./ht run -ss bogus.ht
ec=$?
[[ $ec == 3 ]]  || { echo "FAIL: bogus.ht got exit code of $ec, want 3"; exit 1; }


echo
echo "Exit Code of 'ht exec'"
echo "====================="

./ht exec -ss -only 1 exit.suite
ec=$?
[[ $ec == 0 ]]  || { echo "FAIL: pass got exit code of $ec, want 0"; exit 1; }

./ht exec -ss -only 1-2 exit.suite
ec=$?
[[ $ec == 1 ]]  || { echo "FAIL: fail got exit code of $ec, want 1"; exit 1; }

./ht exec -ss -only 1-3 exit.suite
ec=$?
[[ $ec == 2 ]]  || { echo "FAIL: error got exit code of $ec, want 2"; exit 1; }

./ht exec -ss -only 1-4 exit.suite
ec=$?
[[ $ec == 3 ]]  || { echo "FAIL: bogus got exit code of $ec, want 3"; exit 1; }






rm -rf pass.ht fail.ht error.ht bogus.ht exit.suite
rm -rf 201?-??-??_??h??m??s


echo
echo "PASS"
exit 0
