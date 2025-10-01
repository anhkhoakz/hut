#!/bin/sh -eu

set -- builds git hub lists man meta pages paste todo
for service in "$@"; do
    url="https://git.sr.ht/~sircmpwn/$service.sr.ht"
    dir="$service"srht

    api_dir="/api"
    if [ "$service" = pages ]; then
        api_dir=""
    fi

    tag=$(git -c 'versionsort.suffix=-' ls-remote --refs --sort='version:refname' --tags "$url" |
        tail --lines=1 |
        cut --delimiter='/' --fields=3)
    echo "$url/blob/$tag$api_dir/graph/schema.graphqls"
    curl -f -o "srht/$dir/schema.graphqls" "$url/blob/$tag$api_dir/graph/schema.graphqls"
done
