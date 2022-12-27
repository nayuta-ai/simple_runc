#!/bin/bash
test_nsenter() {
  input="$1"

  gcc ../clone/clone.c
  ./../clone/a.out "$1" &
  sleep 5

  out=$(ps aux)
  lines=$(echo -e "$out" | sed -n '/a.out/p')
  line=$(echo -e "$lines" | sed -n '$p')
  pid=$(echo $line | grep -oE '\b\w+\b' | awk 'NR==2 {print $0}')

  echo "The extracted pid is: $pid"
  gcc nsenter.c
  ./a.out /proc/$pid/ns/uts /bin/bash
}

test_nsenter brosser
