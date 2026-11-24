# Your keyring is locked

On Linux, Project B keeps the key to your saved passwords, passkeys and sync in your
desktop's keyring: on GNOME, the one Passwords and Keys shows; on KDE, KDE Wallet. While
the keyring is locked, Project B can't open them, and Settings says "Your keyring is
locked". Nothing is lost: everything is still there once the keyring opens.

## Unlock it

When Project B needs the key, your desktop asks for your login password. Type it and press
Unlock. If you pressed Cancel, press Try again in Settings, and it asks again.

A computer that logs you in by itself leaves the keyring locked, so the question comes the
first time an app needs it: with sync on, about ten seconds after Project B starts.

## It doesn't ask

GNOME's keyring can stop answering every app until you log in again. It happens when an
app closes while the keyring's unlock question is on screen, and the question is answered
after. Project B waits for the answer rather than give up, even when you close its window,
so this happens only when it's closed by force or crashes, or when another app closes on
its question.

To mend it, log out and back in. Or, without logging out, run this in a terminal, then
press Try again:

```
systemctl --user restart gnome-keyring-daemon
```
