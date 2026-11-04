#!/bin/sh
# Frees disk space on a GitHub-hosted Linux runner before the gate
# (ADR-0032, C-104 addendum 3): the cargo targets of the gate (SDK workspace
# with sanitizer builds, config service, registry, Go SDK with liboec_ffi)
# do not fit into the free space of the ubuntu-24.04 image. Deletes
# preinstalled toolchains the gate does not use (Android SDK, .NET, Haskell,
# Swift, CodeQL bundle). Does nothing elsewhere: not on macOS, not locally,
# not in the container of linux.sh.
set -eu
if [ "${RUNNER_ENVIRONMENT:-}" != github-hosted ] || [ "$(uname -s)" != Linux ]; then
	echo "disk: not a GitHub-hosted Linux runner, nothing to do"
	exit 0
fi
df -h / | tail -1 | awk '{print "disk: before, free " $4}'
for d in /usr/local/lib/android /usr/share/dotnet /opt/ghc /usr/local/.ghcup /usr/share/swift /opt/hostedtoolcache/CodeQL; do
	if [ -e "$d" ]; then
		sudo rm -rf "$d"
		echo "disk: removed $d"
	fi
done
df -h / | tail -1 | awk '{print "disk: after, free " $4}'
