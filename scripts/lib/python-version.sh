# shellcheck shell=bash
# Projection of the canonical bd version (cmd/bd/version.go, semver) onto the
# PEP 440 version carried by the MCP Python package (pyproject.toml,
# __init__.py, uv.lock). Sourced by update-versions.sh, which writes the
# projected form, and check-versions.sh, which gates it, so the two can never
# disagree on the mapping.
#
# Accepted canonical forms, and nothing else:
#   X.Y.Z        -> X.Y.Z
#   X.Y.Z-rc.N   -> X.Y.ZrcN     (PEP 440 pre-release; X.Y.Z-rcN also accepted)
#   X.Y.Z-fd.N   -> X.Y.Z+fd.N   (PEP 440 local version)
#
# Fork releases (-fd.N) are downstream-patched builds of upstream X.Y.Z. PEP
# 440 reserves the local version segment for exactly that: X.Y.Z+fd.N sorts
# after X.Y.Z, fd.N segments compare numerically, and it can never collide with
# a post-release upstream might publish. The alternative X.Y.Z.postN would claim
# upstream's own post-release namespace; the only thing local versions cannot
# do is be uploaded to PyPI, and this fork publishes to its GitHub Releases
# only. The npm package keeps the semver form, which is what its postinstall
# resolves the release tag from.
#
# Any other input is an error: the caller must not write or compare a Python
# version it cannot derive.

python_version() {
    if [ $# -ne 1 ]; then
        echo "python_version: expected exactly one version argument, got $#" >&2
        return 2
    fi
    local version=$1
    if [[ $version =~ ^([0-9]+\.[0-9]+\.[0-9]+)$ ]]; then
        printf '%s\n' "${BASH_REMATCH[1]}"
    elif [[ $version =~ ^([0-9]+\.[0-9]+\.[0-9]+)-rc\.?([0-9]+)$ ]]; then
        printf '%src%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"
    elif [[ $version =~ ^([0-9]+\.[0-9]+\.[0-9]+)-fd\.([0-9]+)$ ]]; then
        printf '%s+fd.%s\n' "${BASH_REMATCH[1]}" "${BASH_REMATCH[2]}"
    else
        echo "python_version: '$version' has no PEP 440 projection (expected X.Y.Z, X.Y.Z-rc.N or X.Y.Z-fd.N)" >&2
        return 1
    fi
}
