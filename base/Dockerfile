# Base image for the Kusari CLI container.
#
# WHY THIS EXISTS
#
# The CLI shells out to git -- generateDiff, createMeta and packageDirectory all
# depend on it -- so an image without git cannot scan at all. It fails with
# `git: not found` before the scan even starts. Until now every consumer worked
# around that by installing git at run time, which is both the wrong shape and
# actively fragile: the image is digest-pinned while apk serves a rolling
# repository, so the two drift apart. That is exactly what broke every
# downstream pipeline when the repository's jq moved to a build requiring
# GLIBC_2.44 while pinned images were frozen at glibc 2.43:
#
#   jq: /usr/lib/libm.so.6: version `GLIBC_2.44' not found
#
# Resolving these packages here, once, freezes a verified set into the published
# image. A bad resolution then breaks this build -- ours, loudly, before ko
# publishes anything -- instead of breaking every consumer silently.

# Deliberately :latest, matching .ko.yaml, so the base keeps receiving upstream
# security updates. What makes that safe is not a pin but the verification step
# below: nothing else in this repository pins base images, and Chainguard does
# not document how long historical digests for free images stay pullable.
FROM cgr.dev/chainguard/wolfi-base:latest

# git is required by the CLI itself.
# jq is used by the CI templates to read SARIF scan output.
#
# Neither is version-pinned. Resolving them against the same repository snapshot
# the base was built from is the point: apk picks a consistent set and the
# verification below proves it runs. A version pin would freeze a workaround in
# place and rot when that exact release leaves the repository.
#
# If this build ever fails at the verification step, an upstream package is
# genuinely broken. Pin the offending package to the last working version HERE,
# with a comment saying why, and remove the pin once upstream recovers.
#
# Deliberately NOT installed: tar and bzip2 (already present in the base, and
# the CLI writes tar.bz2 in pure Go) and libstdc++ (the CLI is built
# CGO_ENABLED=0).
RUN apk add --no-cache git jq

# Prove the binaries actually work, not merely that they unpacked. A package
# that installs cleanly and then dies on a missing symbol is precisely the
# failure this image exists to prevent, and with the base unpinned this step is
# the only thing standing between an upstream change and every consumer.
#
# git is exercised through a real repository operation rather than --version,
# because that is what the CLI actually does with it, and jq through a boolean
# comparison, because that is what the CI templates do with it.
RUN set -eux; \
    git --version; \
    jq --version; \
    tmp="$(mktemp -d)"; \
    git -C "$tmp" init -q; \
    git -C "$tmp" -c user.email=t@example.com -c user.name=t commit -q --allow-empty -m verify; \
    git -C "$tmp" rev-parse HEAD > /dev/null; \
    rm -rf "$tmp"; \
    echo '{"should_proceed":false}' | jq -e '.should_proceed == false' > /dev/null; \
    echo "base image verified"
