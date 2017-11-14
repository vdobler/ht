#! /bin/bash
#
# test-exec.bash tests basic suite execution
#

set -euo pipefail

# ----------------------------------------------------------------------------
#

cat > get3.suite <<EOF
{
    Name: "Make three arbitrary GET request",
    Description: "",
    KeepCookies: true,

    Setup:    {File: "get.ht", Variables: {URL: "{{URL1}}"} },
    Main:     {File: "get.ht", Variables: {URL: "{{URL2}}"} },
    Teardown: {File: "get.ht", Variables: {URL: "{{URL2}}"} }, 

    Variables: {
         URL1:       "https://www.heise.de",
         URL2:       "https://www.reddit.com",
         URL3:       "https://www.google.ch",
    },
}
EOF

cat > get.ht <<EOF
{
    Name: "GET Request {{URL}}",
    Request: { URL: "{{URL}}" },
    Checks: [ {Check: "StatusCode", Expect: 200}  ]
}
EOF


echo
echo "General Suite Execution"
echo "======================="

rm -rf three
./ht exec -ss -output three get3.suite get3.suite get3.suite || \
    (echo "FAIL: exec get3.suite $?"; exit 1;)

tree three > got
cat > want <<EOF
three
├── 1_Make_three_arbitrary_GET_request
│   ├── cookies.json
│   ├── junit-report.xml
│   ├── Main-01.ResponseBody.html
│   ├── _Report_.html
│   ├── result.txt
│   ├── Setup-01.ResponseBody.html
│   ├── Teardown-01.ResponseBody.html
│   └── variables.json
├── 2_Make_three_arbitrary_GET_request
│   ├── cookies.json
│   ├── junit-report.xml
│   ├── Main-01.ResponseBody.html
│   ├── _Report_.html
│   ├── result.txt
│   ├── Setup-01.ResponseBody.html
│   ├── Teardown-01.ResponseBody.html
│   └── variables.json
├── 3_Make_three_arbitrary_GET_request
│   ├── cookies.json
│   ├── junit-report.xml
│   ├── Main-01.ResponseBody.html
│   ├── _Report_.html
│   ├── result.txt
│   ├── Setup-01.ResponseBody.html
│   ├── Teardown-01.ResponseBody.html
│   └── variables.json
└── _Report_.html

3 directories, 25 files
EOF

diff -u got want || \
    (echo "FAIL: unexpected output file structure"; exit 1;)

grep -q "Total 9" three/_Report_.html && \
    grep -q "Passed 9" three/_Report_.html || \
    (echo "FAIL: bad report file"; exit 1;)


rm -rf get3.suite get.ht got want three

echo "PASS"
exit 0
