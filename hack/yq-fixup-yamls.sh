#!/bin/sh
# use yq to reformat yaml files to fit yamllint's syntax validity checks
#
# usage:
#   yq-fixup-yamls.sh file.yaml
#   yq-fixup-yamls.sh dirpath
#
set -e
YQ=${YQ:-$(command -v yq)}

_yaml_fixup_file() {
	yaml="$1"

	${YQ} eval --inplace "${yaml}"
}

_yaml_fixup_files() {
	yamls=$(find "$1" -type f -name '*.yml' -or -name '*.yaml')
	for yaml in ${yamls}; do
		_yaml_fixup_file "${yaml}"
	done
}

if [ -f "$1" ]; then
	_yaml_fixup_file "$1"
elif [ -d "$1" ]; then
	_yaml_fixup_files "$1"
fi
