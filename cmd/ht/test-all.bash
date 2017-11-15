#! /bin/bash
#
# test-all.bash is a wrapper combining all the other test-<xyz>.bash scripts
#

set -u

testscripts="test-error.bash
test-exec.bash
test-exitcode.bash
test-mocks.bash
test-variables.bash"

rc=0
for t in $testscripts; do
    if ./$t  > /dev/null 2>&1; then
	echo "PASS  $t"
    else
	echo "FAIL  $t"
	rc=1
    fi
done

if [[ "$rc" == 1 ]]; then
    echo "FAIL"
    exit 1
fi

echo "PASS"
exit 0


