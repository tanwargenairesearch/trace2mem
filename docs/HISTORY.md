# Commit identity migration

On 2026-09-08, the maintainer updated earlier commit author and committer metadata to the repository's `tanwargenairesearch` identity. Every rewritten commit retains exactly the same source tree; the application, fixtures and recorded measurements are unchanged.

Historical reports and frozen evaluation records retain their original source hashes. [This commit map](commit-map.json) maps those original commits to the equivalent commits after the metadata migration. Binary hashes continue to describe the actual binaries used in those runs; rebuilding a binary with different VCS metadata need not reproduce its hash.

If you cloned before the history replacement, clone again or preserve any local work before realigning your branch with the new remote history.
