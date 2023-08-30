# OpenActa/Haystack

This is the repository for the OpenActa/Haystack storage system.
The code is licensed under AGPL v3+, the docs under CC-BY-SA 4.0.

Haystack is part of the OpenActa project, which is under development.
Haystack is a key-value store with interlinking, so effectively all related
fields are indexed as well as timestamped. However, it is also write-once and
immutable. All of this makes it ideal for storing log data; which, not by
coincidence, is exactly what it was designed for!

Your ideas, bug reports and contributions are most welcome!
Pull requests are also strongly encouraged.

See docs/ for architectural detail and background.

It is still early days. As OpenActa is getting developed, some changes in the
externally accessible functions is expected.

The code contains some *nix specifics such as / filepaths and syslog support,
so won't run under Windows.
Various evolved parts (mem2disk.go comes to mind) require refactoring by now,
particularly since the introduction of Go routines (a diskwriter thread).

    -- Arjen, June 2023
