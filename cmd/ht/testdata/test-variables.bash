#! /bin/bash
#
# test-variables.bash tests variable handling of cmd/ht
#

set -euo pipefail

# ----------------------------------------------------------------------------
#
# Test the following things:
#   - Reading variables from file with -Dfile
#   - Setting variables via -D
#   - Command line set variables overwrite suite variables
#   - Later -D overwrite earlier ones
#   - Variables are substututed in the tests
#   - Variables are extracted
#   - All variables get dumped
#   - Cookies get dumped
#

cat > suite1.suite <<EOF
{
    Name: "First Suite",
    Main: [
        {File: "req1.ht", Variables: {VAR_Y: "call-Y", VAR_Z: "{{VAR_A}}"} }
        {File: "req2.ht"}
    ],
    KeepCookies: true,
    Variables: {
        VAR_A: "suite-A",
        VAR_B: "suite-B",
        VAR_C: "suite-C",
        VAR_W: "{{VAR_U}}",
    }
}
EOF

cat > req1.ht <<EOF
{
    Name: "First Request",
    Request: { URL: "http://httpbin.org/get?a={{VAR_A}}&b={{VAR_B}}&c={{VAR_C}}&d=remote-D&x={{VAR_X}}&y={{VAR_Y}}&z={{VAR_Z}}&w={{VAR_W}}" },
    Checks: [
        {Check: "StatusCode", Expect: 200},
        {Check: "JSON", Element: "args.a", Equals: "\"suite-A\""},
        {Check: "JSON", Element: "args.b", Equals: "\"file-B\""},
        {Check: "JSON", Element: "args.c", Equals: "\"cmdline-C\""},
        {Check: "JSON", Element: "args.d", Equals: "\"remote-D\""},
        {Check: "JSON", Element: "args.x", Equals: "\"x\""},
        {Check: "JSON", Element: "args.y", Equals: "\"call-Y\""},
        {Check: "JSON", Element: "args.z", Equals: "\"suite-A\""},
        {Check: "JSON", Element: "args.w", Equals: "\"cmdline-U\""},
    ],
    VarEx: {
        VAR_D: {Extractor: "BodyExtractor", Regexp: "remote-."},
    }
    Variables: {
        VAR_X: "x"
        VAR_Y: "y"
    }
}
EOF

cat > req2.ht <<EOF
{
    Name: "Second Request",
    Request: {
        URL: "http://httpbin.org/cookies/set?foo=bar",
        FollowRedirects: true,
    },
    Checks: [
        {Check: "JSON", Element: "cookies.foo", Equals: "\"bar\""},
    ],
}
EOF


cat > vars1.json <<EOF
{
    "VAR_B": "file-B",
    "VAR_C": "file-C"
}
EOF


echo
echo "First Test"
echo "=========="

../ht exec -Dfile vars1.json -D VAR_C=cmdline-C \
    -vardump vars2.json -D VAR_U=cmdline-U \
    -cookiedump cookies.json suite1.suite || \
    (echo "FAIL: First suite returned $?"; exit 1;)

# check dumped vars2.json file for proper content
grep -q '"VAR_A": "suite-A"' vars2.json && \
    grep -q '"VAR_B": "file-B"' vars2.json  && \
    grep -q '"VAR_C": "cmdline-C"' vars2.json  && \
    grep -q '"VAR_D": "remote-D"' vars2.json && \
    grep -q '"VAR_U": "cmdline-U"' vars2.json || \
    (echo "FAIL: Bad vars2.json"; exit 1;)

# check dumped cookies.json file for proper content
grep -q '"Name": "foo"' cookies.json && \
    grep -q '"Value": "bar"' cookies.json  && \
    grep -q '"Domain": "httpbin.org"' cookies.json  && \
    grep -q '"Path": "/"' cookies.json || \
    (echo "FAIL: Bad cookies.json"; exit 1;)

# ----------------------------------------------------------------------------
#
# Test the following:
#   - Dumped variables can be used as argument to -Dfile
#   - Cookies can be loaded at startup via -cookies

cat > suite2.suite <<EOF
{
    Name: "Second Suite",
    KeepCookies: true,
    Main: [
        {File: "req3.ht"}
    ]
}
EOF

cat > req3.ht <<EOF
{
    Name: "Third Request",
    Request: { URL: "http://httpbin.org/get?a={{VAR_A}}&b={{VAR_B}}&c={{VAR_C}}&d={{VAR_D}}" },
    Checks: [
        {Check: "StatusCode", Expect: 200},
        {Check: "JSON", Element: "args.a", Equals: "\"suite-A\""},
        {Check: "JSON", Element: "args.b", Equals: "\"file-B\""},
        {Check: "JSON", Element: "args.c", Equals: "\"cmdline-C\""},
        {Check: "JSON", Element: "headers.Cookie", Contains: "foo=bar"},
    ],
}
EOF


echo
echo "Second Test"
echo "==========="

../ht exec -Dfile vars2.json -cookies cookies.json suite2.suite || \
    (echo "FAIL: Second suite returned $?"; exit 1;)


# ----------------------------------------------------------------------------
#
# Test the following:
#  - Seeding the random number generator works
#

cat > suite3.suite <<EOF
{
    Name: "Third Suite",
    Main: [
        {File: "req4.ht", Variables: { WANT: "{{WANT1}}" }  }
        {File: "req4.ht", Variables: { WANT: "{{WANT2}}" } }
    ]
}
EOF

cat > req4.ht <<EOF
{
    Name: "Request 4",
    Request: { URL: "http://httpbin.org/get?r={{RANDOM}}" },
    Checks: [
        {Check: "StatusCode", Expect: 200},
        {Check: "JSON", Element: "args.r", Equals: "\"{{WANT}}\""},
    ]
}
EOF


echo
echo "Third Test"
echo "=========="

../ht exec -seed 123 -D WANT1=616249 -D WANT2=505403 suite3.suite || \
    (echo "FAIL: Third suite run 1 returned $?"; exit 1;)

../ht exec -seed 987 -D WANT1=848308 -D WANT2=143250 suite3.suite || \
    (echo "FAIL: Third suite run 2 returned $?"; exit 1;)

# ----------------------------------------------------------------------------
#
# Test the following:
#  - RANDOM and COUNTER variables work properly
#

cat > suite4.suite <<EOF
{
    Name: "Forth Suite",
    Main: [
        {File: "req5.ht", Variables: { C2: "{{COUNTER}}", R2: "{{RANDOM}}" } }
        {File: "req6.ht", Variables: { C2: "{{COUNTER}}", R2: "{{RANDOM}}" } }
    ],
    Variables: {
        R1: "{{RANDOM}}",
        C1: "{{COUNTER}}",
    }
}
EOF

cat > req5.ht <<EOF
{
    Name: "Request 5",
    Request: { URL: "http://httpbin.org/get?c1={{C1}}&r1={{R1}}&c2={{C2}}&r2={{R2}}&c3={{C3}}&r3={{R3}}" },
    Variables: {
        R3: "{{RANDOM}}"
        C3: "{{COUNTER}}"
    }
    Checks: [
        {Check: "StatusCode", Expect: 200},
        {Check: "JSON", Element: "args.c1", Equals: "\"1\""},
        {Check: "JSON", Element: "args.c2", Equals: "\"2\""},
        {Check: "JSON", Element: "args.c3", Equals: "\"2\""},
        {Check: "JSON", Element: "args.r1", Equals: "\"169035\""},
        {Check: "JSON", Element: "args.r2", Equals: "\"616249\""},
        {Check: "JSON", Element: "args.r3", Equals: "\"616249\""},
    ],
}
EOF

cat > req6.ht <<EOF
{
    Name: "Request 6",
    Request: { URL: "http://httpbin.org/get?c1={{C1}}&r1={{R1}}&c2={{C2}}&r2={{R2}}&c3={{C3}}&r3={{R3}}" },
    Variables: {
        R3: "{{RANDOM}}"
        C3: "{{COUNTER}}"
    }
    Checks: [
        {Check: "StatusCode", Expect: 200},
        {Check: "JSON", Element: "args.c1", Equals: "\"1\""},
        {Check: "JSON", Element: "args.c2", Equals: "\"3\""},
        {Check: "JSON", Element: "args.c3", Equals: "\"3\""},
        {Check: "JSON", Element: "args.r1", Equals: "\"169035\""},
        {Check: "JSON", Element: "args.r2", Equals: "\"505403\""},
        {Check: "JSON", Element: "args.r3", Equals: "\"505403\""},
    ],
}
EOF

echo
echo "Forth Test"
echo "=========="

../ht exec -seed 123 suite4.suite || \
    (echo "FAIL: Fourth suite returned $?"; exit 1;)



rm -f suite1.suite suite2.suite suite3.suite suite4.suite
rm -f req1.ht req2.ht req3.ht req4.ht req5.ht
rm -f vars1.json vars2.json cookies.json


echo "PASS"
exit 0
