#!/usr/bin/env bash
# Build a custom rclone binary with two small patches so that interactive OAuth (adding a
# remote such as Google Drive via the web GUI) works through the session proxy. rclone's
# OAuth callback webserver is made to bind to all interfaces (0.0.0.0:53682) and its
# redirect URI is set to the site's public "/rclone/oauth2callback" URL, so a user's
# browser can be redirected back through the session proxy, which forwards the callback
# into the user's own desktop container.
#
# We don't maintain a fork - we just clone / pull the current upstream rclone and apply
# the two changes as simple search-and-replace strings here, at build time.
#
# Usage: build.sh <rcloneOauthRedirectUrl>
#   <rcloneOauthRedirectUrl> e.g. https://users.example.com/rclone/oauth2callback

RCLONE_OAUTH_REDIRECT_URL="${1:-}"

if [ -z "$RCLONE_OAUTH_REDIRECT_URL" ]; then
    echo "No rclone OAuth redirect URL supplied." >&2
    exit 1
fi

# The folder this script lives in; the built binary is written alongside it.
BUILDDIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
OUTBIN="$BUILDDIR/rclone"
SRCDIR="$BUILDDIR/src"

# rclone is built with Go - we need it on the PATH.
if ! command -v go >/dev/null 2>&1; then
    echo "Go is required to build rclone." >&2
    exit 1
fi

# Refresh the rclone source tree (no fork - just the upstream repository, patched at build time).
if [ ! -d "$SRCDIR/.git" ]; then
    git clone --depth 1 https://github.com/rclone/rclone "$SRCDIR"
else
    git -C "$SRCDIR" pull --ff-only
fi

# Patch 1: bind the OAuth callback webserver to all interfaces so the session proxy container can reach it.
# (Upstream binds to 127.0.0.1:53682 only, which is unreachable from outside the container.)
sed -i 's|bindAddress = "127.0.0.1:" + bindPort|bindAddress = "0.0.0.0:" + bindPort|' "$SRCDIR/lib/oauthutil/oauthutil.go"

# Patch 2: point the OAuth redirect URI at our public "/rclone/oauth2callback" endpoint instead of the
# hard-coded "http://127.0.0.1:53682/", so the user's browser gets redirected back to the session proxy,
# which forwards the callback into their desktop container.
sed -i "s|RedirectURL = \"http://\" + bindAddress + \"/\"|RedirectURL = \"${RCLONE_OAUTH_REDIRECT_URL}\"|" "$SRCDIR/lib/oauthutil/oauthutil.go"

# Sanity-check that both patches actually applied (in case the upstream source layout changes).
if ! grep -q 'bindAddress = "0.0.0.0:" + bindPort' "$SRCDIR/lib/oauthutil/oauthutil.go"; then
    echo "Failed to patch rclone bind address (source layout may have changed)." >&2
    exit 1
fi
if ! grep -q "RedirectURL = \"${RCLONE_OAUTH_REDIRECT_URL}\"" "$SRCDIR/lib/oauthutil/oauthutil.go"; then
    echo "Failed to patch rclone redirect URL (source layout may have changed)." >&2
    exit 1
fi

# Build rclone as a self-contained binary suitable for copying into the desktop container.
( cd "$SRCDIR" && go build -trimpath -ldflags="-s -w" -o "$OUTBIN" )

if [ ! -f "$OUTBIN" ]; then
    echo "Failed to build rclone." >&2
    exit 1
fi

echo "Built custom rclone to $OUTBIN"
