#!/usr/bin/env python3
# Patch the embedded rclone web GUI dist.zip so its React Router uses a basename
# matching the sub-path it's served behind (e.g. "/rclone"). The GUI is a compiled
# React single-page app built with a hard-coded router basename of "/", so when it's
# proxied behind a path prefix the browser URL (e.g. "/rclone/login") no longer matches
# any client-side route and the app shows "Unexpected Application Error! 404 Not Found".
#
# We don't maintain a fork of the GUI - we just modify the compiled router factory in
# the pre-built dist bundle at build time (in the same spirit as the oauthutil patches
# in build.sh). We look for the minified router factory and default its basename to the
# supplied prefix.
#
# Usage: patch-gui-dist.py <dist.zip> <basename>
#   <basename>  e.g. /rclone  (the path prefix the GUI is served behind)

import sys
import zipfile


def main() -> int:
    if len(sys.argv) != 3:
        print("Usage: patch-gui-dist.py <dist.zip> <basename>", file=sys.stderr)
        return 2

    zip_path, basename = sys.argv[1], sys.argv[2]
    if not basename.startswith("/"):
        print(f"basename must start with '/', got {basename!r}", file=sys.stderr)
        return 2

    # The minified React Router data-router factory that the app calls without options,
    # so its basename option is undefined and falls back to "/". We default it to our prefix.
    # Careful: react-router may reference a config's basename elsewhere; this exact string
    # (the factory default) appears exactly once in the bundle.
    search = "basename:n?.basename,"
    replace = f'basename:n?.basename||"{basename}",'

    src = zipfile.ZipFile(zip_path, "r")
    names = src.namelist()

    replacements = 0
    new_files = []
    for name in names:
        data = src.read(name)
        if name.endswith(".js"):
            text = data.decode("utf-8")
            n = text.count(search)
            if n:
                text = text.replace(search, replace)
                replacements += n
                data = text.encode("utf-8")
        info = src.getinfo(name)
        new_files.append((info, data))
    src.close()

    if replacements != 1:
        print(
            f"Expected exactly 1 basename replacement in GUI dist, found {replacements}. "
            "The GUI bundle layout may have changed - refusing to write a possibly-broken dist.",
            file=sys.stderr,
        )
        return 1

    dst = zipfile.ZipFile(zip_path, "w", zipfile.ZIP_DEFLATED)
    for info, data in new_files:
        dst.writestr(info, data)
    dst.close()

    print(f"Patched {zip_path}: set React Router basename to {basename!r}")
    return 0


if __name__ == "__main__":
    sys.exit(main())
