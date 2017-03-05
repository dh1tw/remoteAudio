#!/bin/bash

num=0
for t in `git tag`
do
  if [ "$num" -ge 120 ]
    then
      break
  fi
  git push origin :$t
  git tag -d $t
  num=`expr $num + 1`
  echo "Removed $t"
done```
