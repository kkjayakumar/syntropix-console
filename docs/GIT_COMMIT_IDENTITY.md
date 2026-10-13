# Git commit identity (Syntropix OSS)

Commits on this repo must map to the **`kkjayakumar`** GitHub account only.

## Required local git config

```bash
git config --global user.name "kkjayakumar"
git config --global user.email "12824735+kkjayakumar@users.noreply.github.com"
```

Do **not** use employer or secondary GitHub-linked emails (they can attach the wrong GitHub profile to commits).

## Cursor / IDE

Cursor adds `Co-authored-by: Cursor <cursoragent@cursor.com>` unless you disable it. That creates a **cursoragent** contributor on GitHub.

1. Turn off commit attribution / co-author in Cursor settings.
2. For commits on this repo, disable hooks when needed:

```bash
git -c core.hooksPath=.git/empty-hooks commit ...
```

(`.git/empty-hooks` is an empty directory in this clone.)

## Verify before push

```bash
git log -1 --format=fuller
```

Author and committer should be `kkjayakumar` with the `@users.noreply.github.com` address above.
