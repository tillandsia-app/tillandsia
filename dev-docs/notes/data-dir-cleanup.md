# Data Directory Cleanup

Each deploy gets an isolated data dir (`/var/lib/tillandsia/<app>/data-<timestamp>`)
to prevent SQLite lock conflicts between container versions. Old dirs are cleaned
up during the deploy pipeline (last 3 kept).

This is a band-aid. The cleanup only runs on success — a failed deploy leaks dirs.
The right fix is to move cleanup to a cron/systemd timer on the server:

```
# /etc/cron.daily/tillandsia-cleanup
find /var/lib/tillandsia/*/data-* -maxdepth 1 -type d | sort | head -n -3 | xargs -r rm -rf
```

For now the pipeline approach is fine — just something to fix before we hit
production.