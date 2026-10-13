# Git commit identity (Syntropix OSS)

Commits on this repo must map to the **`kkjayakumar`** GitHub account only.

## Required local git config

```bash
git config --global user.name "kkjayakumar"
git config --global user.email "12824735+kkjayakumar@users.noreply.github.com"
```

Do **not** use employer or secondary GitHub-linked emails (they can attach the wrong GitHub profile to commits).

## Cursor / IDE

If commits show `Co-authored-by: Cursor <cursoragent@cursor.com>`, disable automatic co-author trailers in Cursor settings before committing to this repository.

## Verify before push

```bash
git log -1 --format=fuller
```

Author and committer should be `kkjayakumar` with the `@users.noreply.github.com` address above.
