#!/usr/bin/env bash
# Build a custom rclone binary with small patches so that interactive OAuth (adding a
# remote such as Google Drive via the web GUI) works through the session proxy, and so the
# embedded web GUI can be served behind a path prefix. Patches applied at build time:
#   - bind the OAuth callback webserver to all interfaces (0.0.0.0:53682)
#   - point the OAuth redirect URI at the site's "/rclone/oauth2callback" endpoint
#   - set the embedded GUI's React Router basename to "/rclone" so its client-side routing
#     works when it's served behind the "/rclone" path by the session proxy
#
# We don't maintain a fork - we just clone / pull the current upstream rclone (and its
# pre-built GUI bundle) and apply these changes as simple search-and-replace steps here,
# at build time.
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

# Go's default build work directory and cache live under /tmp, which is often a small tmpfs
# and can run out of space when building something as large as rclone. Point them at the main
# disk (alongside this script) instead so the build doesn't fail with "no space left on device".
BUILDTMP="$BUILDDIR/tmp"
GOCACHE="$BUILDDIR/gocache"
mkdir -p "$BUILDTMP" "$GOCACHE"
export TMPDIR="$BUILDTMP"
export GOCACHE="$GOCACHE"

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

# Patch 3: the embedded web GUI (dist.zip) is a compiled React SPA built with a hard-coded React Router
# basename of "/". When the session proxy serves it behind the "/rclone" path prefix the browser URL no
# longer matches any client-side route, so we default the router basename to the "/rclone" prefix at build
# time. This bakes the fix into the embedded bundle (no runtime rewriting needed).
# Note: this relies on the minified router factory string; if the GUI bundle layout changes the script
# refuses to write a possibly-broken dist and fails the build loudly.
if command -v python3 >/dev/null 2>&1; then
    python3 "$BUILDDIR/patch-gui-dist.py" "$SRCDIR/cmd/gui/dist.zip" "/rclone"
    if [ $? -ne 0 ]; then
        echo "Failed to patch GUI dist basename." >&2
        exit 1
    fi
else
    echo "python3 not found - skipping GUI dist basename patch (GUI routing behind /rclone will be broken)." >&2
fi

# Build rclone as a self-contained binary suitable for copying into the desktop container.
# Build to a temporary file first so a failed build can't leave a stale binary in place that
# later steps would silently use.
OUTBIN_TMP="$OUTBIN.tmp.$$"
( cd "$SRCDIR" && go build -trimpath -ldflags="-s -w" -o "$OUTBIN_TMP" )
BUILD_RC=$?

if [ $BUILD_RC -ne 0 ] || [ ! -f "$OUTBIN_TMP" ]; then
    echo "Failed to build rclone." >&2
    rm -f "$OUTBIN_TMP"
    exit 1
fi

mv "$OUTBIN_TMP" "$OUTBIN"
echo "Built custom rclone to $OUTBIN"
