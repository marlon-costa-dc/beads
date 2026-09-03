---
title: "bd mail"
description: "Delegates mail operations to an external mail provider."
---

{/* AUTO-GENERATED: do not edit manually */}

Generated from `bd help --doc mail`.

Delegates mail operations to an external mail provider.

Agents often type 'bd mail' when working with beads, but mail functionality
is typically provided by the orchestrator. This command bridges that gap
by delegating to the configured mail provider.

Configuration (checked in order):
  1. BEADS_MAIL_DELEGATE or BD_MAIL_DELEGATE environment variable
  2. 'mail.delegate' config setting (bd config set mail.delegate "gc mail")

Examples:
  # Configure delegation (one-time setup)
<<<<<<< HEAD:website/versioned_docs/version-1.1.0/cli-reference/mail.md
  export BEADS_MAIL_DELEGATE="gc mail"
=======
  `export BEADS_MAIL_DELEGATE="gt mail"`
>>>>>>> origin/main:docs/cli-reference/mail.md
  # or
  bd config set mail.delegate "gc mail"

  # Then use bd mail as if it were gc mail
  bd mail inbox                    # Lists inbox
  bd mail send mayor/ -s "Hi"      # Sends mail
  bd mail read msg-123             # Reads a message

```
bd mail [subcommand] [args...] [flags]
```
