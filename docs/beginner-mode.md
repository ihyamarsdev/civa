# Beginner Mode (`civa start`)

`civa start` is the beginner entrypoint for `civa`.

It opens a guided wizard and routes you to one of these flows:

- `setup` — install your public key on a fresh server
- `plan init` — generate a reusable plan interactively
- `help` — open command help
- `exit` — leave the wizard without running anything

`civa start` only runs in an interactive terminal. In non-interactive/scripted sessions, use direct commands such as `civa setup`, `civa plan init`, or `civa help`.

## Why use beginner mode

- You can start without memorizing flags.
- Inputs are shown with safe defaults so you can press Enter to keep them.
- It keeps using the same backend command flow (`setup` and `plan init`), so behavior stays consistent.

## Wizard defaults you can keep

The wizard pre-fills common starter values, including:

- first server address: `203.0.113.10`
- first hostname: `web-01`
- additional server examples: `203.0.113.11`, `203.0.113.12`, ...
- additional hostnames: `node-02`, `node-03`, ...
- SSH port prompt pre-filled from current CLI default (`22` unless changed)

You can overwrite any value at prompt time.

## Typical first-run sequence

1. Run the wizard:

```bash
civa start
```

2. Choose **Set up SSH access first (recommended, default)** to run `setup`.

3. After setup succeeds, run `civa start` again and choose **Create an execution plan step-by-step**.

4. Review the generated plan with:

```bash
civa plan review <plan-name>
```

5. Apply when ready:

```bash
civa apply <plan-name>
```

## If you prefer direct commands

Beginner mode is optional. You can always run commands directly:

```bash
civa setup --server <ip> --ssh-user <user> --ssh-public-key <path>
civa plan init
```
