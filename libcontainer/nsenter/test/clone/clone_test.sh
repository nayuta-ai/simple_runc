#!/bin/bash
assert() {
  input="$1"

  gcc clone.c
  actual=$(./a.out $1)

  if ! echo "$actual" | grep -q 'clone() returned'; then
    echo "Error: Expected string 'clone() returned' not found in output"
    exit 1
  fi
  if ! echo "$actual" | grep -q 'uts.nodename in parent:'; then
    echo "Error: Expected string 'uts.nodename in parent:' not found in output"
    exit 1
  fi
  if ! echo "$actual" | grep -q 'a.out'; then
    echo "Error: Expected string 'a.out' not found in output"
    exit 1
  fi
  if ! echo "$actual" | grep -q 'child has terminated'; then
    echo "Error: Expected string 'child has terminated' not found in output"
    exit 1
  fi
  echo "$input => $actual"
}

assert "-t brosser"

echo OK