#!/bin/bash

while true; do
	# mono /pba/binaries/StorylineSummarizationHost/bin/Release/StorylineSummarizationHost.exe /pba/config.json
	cd /root/summarizer
	./app.py
	>&2 echo "summarization server exited with code: $?"
	>&2 echo "sleep 3 seconds, hit CTRL+C to skip restart and terminate"
	sleep 3
	[ $? -ne 0 ] && >&2 echo "terminating" && exit 1
	>&2 echo "restarting"
done
