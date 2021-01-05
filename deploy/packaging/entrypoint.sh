#!/usr/bin/env sh


set -e

# check to see if this file is being run or sourced from another script
_is_sourced() {
	# https://unix.stackexchange.com/a/215279
	[ "${#FUNCNAME[@]}" -ge 2 ] \
		&& [ "${FUNCNAME[0]}" = '_is_sourced' ] \
		&& [ "${FUNCNAME[1]}" = 'source' ]
}

# check arguments for an option that would cause /themis to stop
# return true if there is one
_want_help() {
	local arg
	for arg; do
		case "$arg" in
			-'?'|--help|-v)
				return 0
				;;
		esac
	done
	return 1
}

_main() {
	# if command starts with an option, prepend themis
	if [ "${1:0:1}" = '-' ]; then
		set -- /themis "$@"
	fi
		# skip setup if they aren't running /themis or want an option that stops /themis
	if [ "$1" = '/themis' ] && ! _want_help "$@"; then
		echo "Entrypoint script for themis Server ${VERSION} started."

		if [ ! -s /etc/themis/themis.yaml ]; then
		  echo "Building out template for file"
		  /spruce merge /themis.yaml /tmp/themis_spruce.yaml > /etc/themis/themis.yaml
		  cat /etc/themis/themis.yaml
		fi
	fi

	exec "$@"
}

# If we are sourced from elsewhere, don't perform any further actions
if ! _is_sourced; then
	_main "$@"
fi