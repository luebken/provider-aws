#!/bin/bash
echo 

for f in $(find ./apis -name '*_types.go' -not -name 'zz_types.go'); do 
echo $f; 
for tag in `git tag -l`; do [ -n "`git ls-tree $tag $f`" ] && echo $tag; done
done

