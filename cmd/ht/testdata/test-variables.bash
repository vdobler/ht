#! /bin/bash
#
# test-variables.bash tests variable handling of cmd/ht
#

set -euo pipefail

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


# Test the following:
#   - Dumped variables can be used as argument to -Dfile
#   - Cookies can be loaded at startup via -cookies
../ht exec -Dfile vars2.json -cookies cookies.json suite2.suite || \
    (echo "FAIL: Second suite returned $?"; exit 1;)


# ----------------------------------------------------------------------------

cat > suite3.suite <<EOF
{
    Name: "Third Suite",
    Main: [
        {File: "req4.ht", Variables: { C2: "{{COUNTER}}", R2: "{{RANDOM}}" } }
        {File: "req4.ht", Variables: { C2: "{{COUNTER}}", R2: "{{RANDOM}}" } }
    ],
    Variables: {
        R1: "{{RANDOM}}",
        C1: "{{COUNTER}}",
    }
}
EOF

cat > req4.ht <<EOF
{
    Name: "",
    Request: { URL: "http://httpbin.org/get?c1={{C1}}&r1={{R1}}&c2={{C2}}&r2={{R2}}&c3={{C3}}&r3={{R3}}" },
    Checks: [
        {Check: "StatusCode", Expect: 200},
        {Check: "JSON", Element: "args.c1", Equals: "\"suite-A\""},
        {Check: "JSON", Element: "args.r1", Equals: "\"file-B\""},
        {Check: "JSON", Element: "args.c2", Equals: "\"cmdline-C\""},
        {Check: "JSON", Element: "args.r2", Equals: "\"remote-D\""},
        {Check: "JSON", Element: "args.c3", Equals: "\"cmdline-C\""},
        {Check: "JSON", Element: "args.r3", Equals: "\"remote-D\""},
    ],
    Variables: {
        R3: "{{RANDOM}}"
        C3: "{{COUNTER}}"
    }
}
EOF

../ht exec suite3.suite || \
    (echo "FAIL: Third suite returned $?"; exit 1;)



rm -rf suite1.suite suite2.suite suite3.suite req1.ht req2.ht req3.ht req4.ht vars1.json vars2.json cookies.json


echo "PASS"
exit 0
