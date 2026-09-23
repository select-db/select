#!/usr/bin/env bash
# Creates or updates every label a finder issue can carry, the fixer's fix:*
# status included. references/issue.md says when each applies; this is the list.
set -euo pipefail

label() { gh label create "$1" --color "$2" --description "$3" --force >/dev/null; }

label agent:finder         5319e7 "Filed by the dialect finder agent"
label needs-triage         fbca04 "A human or the fixer must decide whether this is a bug"
label bug                  d73a4a "Something isn't working"

label area:permission      1d76db "Which rights a statement requires"
label area:completion      1d76db "What is suggested at a position"
label area:lint            1d76db "Which diagnostics a statement raises"
label area:resolution      1d76db "Hover, references, go-to-definition"

label dialect:postgresql   c5def5 "Reproduces on PostgreSQL"
label dialect:mysql        c5def5 "Reproduces on MySQL"
label dialect:sqlite       c5def5 "Reproduces on SQLite"

label sev:bypass           b60205 "Ran with no grant"
label sev:wrong-right      d93f0b "Ran under the wrong permission"
label sev:unchecked-read   e99695 "The write was checked, a read inside it was not"
label sev:false-denial     f9d0c4 "Refused a statement the user holds the rights for"
label sev:quality          fef2c0 "A wrong suggestion, diagnostic or hover"

label oracle:cross-dialect bfdadc "Dialects disagree on the same case"
label oracle:judgment      bfdadc "Rests on the finder's expectation alone"

label verdict:fixed        0e8a16 "Closed: the code changed"
label verdict:not-a-bug    0e8a16 "Closed: the expectation was wrong; the reason becomes a finder precedent"
label verdict:duplicate    0e8a16 "Closed: duplicates another issue"
label verdict:wontfix      0e8a16 "Closed: real, not worth changing"

label fix:go               0052cc "Approved for the dialect fixer; only a writer's label counts"
label fix:in-progress      c5def5 "The dialect fixer is working on this"
label fix:pr-open          c5def5 "The dialect fixer opened a pull request for this"
label fix:blocked          d93f0b "The dialect fixer disagrees or cannot fix this; see its comment"
label fix:reviewed         c5def5 "The fixer ran the review gate's skills over its pull request"
