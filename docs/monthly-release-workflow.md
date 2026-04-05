# Monthly Release Workflow

This document explains how the monthly binary release process trails Cloud by one month while still allowing guarded patch promotions.

## Monthly Release Flow

```text
                      MONTH N
        merges to main ---------------------------> Cloud runs latest code
             |                                             |
             | every merge builds                          |
             +-----> preview artifact: main-{hash}         |
                                                           |
end of MONTH N --------------------------------------------+
             |
             +-----> create cutoff tag for MONTH N
                     e.g. release-cutoff-2026-03
                     points to frozen main commit Cn

                      MONTH N+1
        merges to main ---------------------------> Cloud runs newer code
             |
             +-----> preview artifact: main-{hash}

end of MONTH N+1
             |
             +-----> create cutoff tag for MONTH N+1
             |       e.g. release-cutoff-2026-04
             |       points to commit Cn+1
             |
             +-----> prepare release PR for previous cutoff
                     release version: v26.3.0
                     target SHA: Cn
                     changelog range: previous stable tag .. Cn
                     manifest stores:
                     - version
                     - target_sha
                     - cutoff_tag
                     - next_cutoff_tag

release PR merged
             |
             +-----> trigger stable tag workflow
                     read manifest
                     create tag v26.3.0 on Cn
                     not on current main HEAD

tag pushed
             |
             +-----> existing release workflow builds binaries for v26.3.0
```

## Patch Promotion Decision Tree

```text
Need to patch 26.3.0?
        |
        v
pick candidate commit on main
        |
        v
is candidate reachable from origin/main?
        |
   +----+----+
   |         |
  no        yes
   |         |
 reject      v
         is candidate ancestor of next cutoff commit?
                    |
               +----+----+
               |         |
              no        yes
               |         |
      too new for 26.3.x  v
      use main-{hash}     create next tag on release line
      or wait             e.g. v26.3.1
```

## Release Lines Meaning

```text
v26.3.0
  = March cutoff
  = released at end of April

v26.3.1
  = optional patch on March line
  = only allowed if commit is already included in April cutoff

v26.4.0
  = April cutoff
  = released at end of May
```
