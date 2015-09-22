#! /bin/bash

checks=$(oracle -pos ./check.go:#332 implements | \
    cut -f 2 | cut -d " " -f 6 | tr -d "*" | sort)

echo '<!DOCTYPE html>'
echo '<html><head><title>Availbale Checks</title><meta charset="UTF-8"></head>'
echo "<body><h1>Available Checks</h1>"

echo "<p>Version: $(git describe)</p>"

for c in $checks Condition; do
    [[ -n "$c" ]] || continue
    if [[ "$c" = "Condition" ]];then
	echo "<hr>"
	echo "<p>Type Condition is not a Check but it is used so often"
	echo "   in checks that it is worth describing here.</p>"
    fi
    echo "<h2>$c</h2>"
    echo "<pre>"
    go doc "$c" \
	| sed -e "s/&/\\&amp;/g; s/</\\&lt;/g; s/>/\\&gt;/g"
    echo "</pre>"
done | grep -v -E "^func "




echo "</body></html>"
