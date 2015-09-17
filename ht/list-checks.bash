#! /bin/bash

checks=$(oracle -pos ./check.go:#332 implements | \
    cut -f 2 | cut -d " " -f 6 | tr -d "*" | sort)


echo "<html><head><title>Checks</title></head>"
echo "<body><h1>Available Checks</h1>"

for c in $checks; do
    [[ -n "$c" ]] || continue
    echo "<h2>$c</h2>"
    go doc "$c" \
	| sed -e 's,^\(type.*{\)$,<pre>\1,' \
	| sed -e 's,^\(type.*\)$,<pre>\1</pre>,' \
	| sed -e "s,^},}</pre>,"
    echo
done | grep -v -E "^func "


echo "</body></html>"
