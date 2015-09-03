#! /bin/bash

checks=$(oracle -pos ./check.go:#332 implements | \
    cut -f 2 | cut -d " " -f 6 | tr -d "*" | sort)


for c in $checks; do
    [[ -n "$c" ]] || continue
    figlet -W -w 100 "$c"
    go doc "$c"

    echo
    echo
done | grep -v -E "^func "

echo "===================================================================================================="

figlet -W -w 100 "Conditon"
go doc Condition | grep -v -E "^func "
