#!/bin/bash

while true; do
	./src/jvm/io/fsq/twofishes/scripts/serve.py -p 8081 --vm_map_count 65530 /data/twofishes/latest/
	r=$?
	[ $r -eq 0 ] && >&2 echo "twofishes exited cleanly" && exit 0
	>&2 echo "twofishes exited with code: $r"
	>&2 echo "sleep 3 seconds, hit CTRL+C to skip restart and terminate"
	sleep 3
	[ $? -ne 0 ] && >&2 echo "terminating" && exit $r
	>&2 echo "restarting"
done
