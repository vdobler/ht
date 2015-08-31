#! /bin/bash

checktypes=$(grep -E "^func \(.*\) Execute\(t \*Test\) error {" *.go \
    | grep -v _test.go: \
    | sed -e "s/\*//; s/)//" \
    | cut -d " " -f 3 | sort)

echo "Available Checks"
echo "================"
echo "$checktypes"

for t in $checktypes; do
    echo
    echo
    sourcefile=$(grep -E -l "^type $t " *.go)
    lines=$(awk  "
BEGIN       { n=1; found=0; start=0; end=0; } 
/^type $t struct {/ { found=1; n++; next; } 
/^type $t / { found=1; end=n; n++; next; } 
/^\$/       { n++; if(found==0){ start = n; } next; }
/^}$/       { if(found==1 && end==0){ end = n; } n++; next;}
            { n++; next; } 
END         { printf \"%d,%dp\",start, end; }" $sourcefile)
    sed -n $lines $sourcefile
    echo "// (in file $sourcefile)"
done
