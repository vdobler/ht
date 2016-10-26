#! /bin/bash

checks=$(guru -scope  . implements "check.go:#332" | sed -n -e 's/.* //; s/*//; 2,$p' | grep -v -E "^[^*A-Z]")

extractors=$(guru -scope  . implements "extractor.go:#501" | sed -n -e 's/.* //; s/*//; 2,$p' | grep -v -E "^[^*A-Z]")

godoc2html() {
    echo "<pre>"
    go doc "$1" \
	| sed -e "s/&/\\&amp;/g; s/</\\&lt;/g; s/>/\\&gt;/g" \
	| grep -v -E "^func " | sed -e "s/\t/        /; s/^    //; s/^}$/}\n/"
    echo "</pre>"
}

{
echo '<!DOCTYPE html>'
echo '<html><head><title>ht: Type Documentation</title><meta charset="UTF-8"></head>'
echo "<body><h1>ht: HTTP Testing</h1>"

echo "<p>ht version $(git describe)</p>"

echo "<h2>Test</h2>"
godoc2html Test

echo "<h3>Request</h2>"
godoc2html Request

echo "<h3>Execution</h2>"
godoc2html Execution

echo "<h2>Checks</h2>"
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
for e in $extractors; do
    [[ -n "$e" ]] || continue
    echo "<h3>$e</h3>"
    godoc2html "$e"
done 


echo "</body></html>"
} > Documentation.html

wkhtmltopdf --minimum-font-size 16 -s A4 -L 24 -R 24 -T 24 -B 24 Documentation.html Documentation.pdf
