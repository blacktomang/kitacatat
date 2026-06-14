#!/usr/bin/env sh
# Thin wrapper around `go` that sets the CGO include/lib paths gosseract needs
# to link Tesseract + Leptonica.
#
# gosseract includes "leptonica/allheaders.h", so the compiler needs the
# *parent* of the leptonica/ dir on its include path. On Homebrew (macOS),
# leptonica is keg-only and lives under `brew --prefix leptonica`, which is not
# on the default search path. On Debian/Linux the apt -dev headers already sit
# in /usr/include, so we add nothing and rely on the defaults.
set -e

export CGO_ENABLED=1

if command -v brew >/dev/null 2>&1; then
  LEPT="$(brew --prefix leptonica 2>/dev/null)"
  TESS="$(brew --prefix tesseract 2>/dev/null)"
  if [ -n "$LEPT" ] && [ -n "$TESS" ]; then
    export CGO_CPPFLAGS="-I${LEPT}/include -I${TESS}/include"
    export CGO_LDFLAGS="-L${LEPT}/lib -L${TESS}/lib"
  fi
fi

exec go "$@"
