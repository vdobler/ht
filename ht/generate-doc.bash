#! /bin/bash

set -eu

checks=$(guru -scope  . implements "check.go:#350" | sed -n -e 's/.* //; s/*//; 2,$p' | grep -v -E "^[^*A-Z]" | sort)

extractors=$(guru -scope  . implements "extractor.go:#466" | sed -n -e 's/.* //; s/*//; 2,$p' | grep -v -E "^[^*A-Z]" | sort)



godoc2html() {
    echo "<pre>"
    go doc "$1" \
	| sed -e "s/&/\\&amp;/g; s/</\\&lt;/g; s/>/\\&gt;/g" \
	| grep -v -E "^func " | sed -e "s/\t/        /; s/^    //; s/^}$/}\n/"
    echo "</pre>"
}

table() {
    r=0
    echo "<table>"
    for c in $*; do
	if [[ $(($r%6)) == 0 ]]; then
	    echo "  <tr>"
	fi
	echo "    <td>$c &nbsp; &nbsp; &nbsp;  &nbsp; <//td>"
	r=$(($r+1))
    done
    echo "</table>"
}

{
echo '<!DOCTYPE html>'
echo '<html><head><title>ht: Type Documentation</title><meta charset="UTF-8"></head>'
echo "<body><h1>ht: HTTP Testing</h1>"

echo "<p>ht version $(git describe)</p>"

echo "<h2>Test</h2>"
godoc2html Test

for t in Request Execution Cookie CheckList ExtractorMap; do
    echo "<h3>$t</h2>"
    godoc2html "$t"
done

echo "<h2>Checks</h2>"
table $checks
for c in $checks; do
    [[ -n "$c" ]] || continue
    echo "<h3>$c</h3>"
    godoc2html "$c"
done 
echo "<hr>"
echo "<h3>Condition</h3>"
echo "<p>Type Condition is not a Check but it is used so often"
echo "   in checks that it is worth describing here.</p>"
godoc2html "Condition"

echo "<h2>Variable Extractors</h2>"
table $extractors
for e in $extractors; do
    [[ -n "$e" ]] || continue
    echo "<h3>$e</h3>"
    godoc2html "$e"
done 

echo "<h2>Hjson Types</h2>"
for t in RawTest RawSuite RawElement RawScenario RawLoadTest RawMock; do
    echo "<h3>$t</h2>"
    godoc2html "github.com/vdobler/ht/suite.$t"
done

echo "<h2>Mocking</h2>"
for t in Mock Mapping; do
    echo "<h3>$t</h2>"
    godoc2html "github.com/vdobler/ht/mock.$t"
done



echo "</body></html>"
} > Documentation.html

wkhtmltopdf --minimum-font-size 16 -s A4 -L 24 -R 24 -T 24 -B 24 Documentation.html Documentation.pdf
