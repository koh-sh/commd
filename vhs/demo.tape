# VHS tape for the README demo GIF. Run with `mise run demo`.
# See vhs/README.md for prerequisites (ttyd + ffmpeg).

Output vhs/demo.gif

Set Shell zsh
Set FontSize 16
Set Width 1100
Set Height 720
Set Theme "Catppuccin Mocha"
Set PlaybackSpeed 1.5
Set TypingSpeed 80ms
Set Padding 20

# --- Setup (not recorded) ------------------------------------------
# CI=true makes Bubble Tea skip the cursor/background-color queries that
# otherwise add a 5s startup delay under VHS' PTY.
Hide
Type `export PATH="$PWD:$PATH"`
Enter
Sleep 100ms
Type `export CI=true`
Enter
Sleep 100ms
Type "clear"
Enter
Sleep 200ms
Show
# -------------------------------------------------------------------

Type "commd review --output stdout internal/markdown/testdata/basic.md"
Sleep 600ms
Enter
Sleep 2.5s

# Navigate from the Overview down to "Step 1: Auth Middleware".
Down
Sleep 700ms

# Add a section-level comment with the default `question` label.
Type "c"
Sleep 1s
Type "Which auth library are we standardizing on?"
Sleep 1.2s
Ctrl+S
Sleep 1.5s

# Move into the JWT verification subsection and add a `suggestion`.
Down
Sleep 700ms
Type "c"
Sleep 1s
# question -> issue -> suggestion (two Shift+Tab steps backward).
Shift+Tab
Sleep 600ms
Shift+Tab
Sleep 800ms
Type "Cache the JWKS to avoid per-request IdP lookups"
Sleep 1.2s
Ctrl+S
Sleep 1.5s

# Submit the review and confirm.
Type "s"
Sleep 1.2s
Type "y"
Sleep 3.5s
