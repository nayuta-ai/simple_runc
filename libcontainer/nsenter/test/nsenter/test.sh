assert() {
  input="$1"
  out=$(uname -n)

  if test "$out" = "$input"; then
    echo "$input => $out"
  else
    echo "$input => $out expected, but got $input"
    exit 1
  fi
}

assert brosser
exit