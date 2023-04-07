#!/bin/sh
# helper script to install build auxiliary tools in local directory
#
# usage:
#   GOBIN=<dir> install-tools.sh --<tool-name>
#
set -e
GO_CMD=${GO_CMD:-$(command -v go)}
PY_CMD="${PY_CMD:-$(command -v python3)}"
GOBIN=${GOBIN:-${GOPATH}/bin}

_require_gobin() {
	mkdir -p "${GOBIN}"
}

_install_tool() {
	GOBIN="${GOBIN}" ${GO_CMD} install "$1"
}

_install_kustomize() {
	_install_tool sigs.k8s.io/kustomize/kustomize/v4@v4.5.2
}

_install_controller_gen() {
	_install_tool sigs.k8s.io/controller-tools/cmd/controller-gen@v0.6.2
}

_install_revive() {
	_install_tool github.com/mgechev/revive@v1.2.3
}

_install_golangci_lint() {
	_install_tool github.com/golangci/golangci-lint/cmd/golangci-lint@v1.46.2
}

_install_yq() {
	_install_tool github.com/mikefarah/yq/v4@v4.23.1
}

_install_gosec() {
	_install_tool github.com/securego/gosec/v2/cmd/gosec@v2.13.1
}

_install_gitlint() {
	_require_gobin
	"${PY_CMD}" -m venv "${GOBIN}/.py"
	"${GOBIN}/.py/bin/pip" install "gitlint==0.19.1"
	ln -s "${GOBIN}/.py/bin/gitlint"  "${GOBIN}/gitlint"
}

case "$1" in
	--kustomize)
		_require_gobin
		_install_kustomize
		;;
	--controller-gen)
		_require_gobin
		_install_controller_gen
		;;
	--revive)
		_require_gobin
		_install_revive
		;;
	--golangci-lint)
		_require_gobin
		_install_golangci_lint
		;;
	--yq)
		_require_gobin
		_install_yq
		;;
	--gosec)
		_require_gobin
		_install_gosec
		;;
	--gitlint)
		_install_gitlint
		;;
	*)
		echo "usage: GOBIN=<dir> $0 --<tool-name>"
		echo ""
		echo "available tools:"
		echo "  --kustomize"
		echo "  --controller-gen"
		echo "  --revive"
		echo "  --golangci-lint"
		echo "  --yq"
		echo "  --gosec"
		echo "  --gitlint"
		;;
esac
