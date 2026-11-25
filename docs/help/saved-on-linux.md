# Who can read your passwords and passkeys on Linux

On Linux, Project B keeps the key to your saved passwords and passkeys in your desktop's
keyring. The keyring opens when you log in with your password, or the first time an app
asks if the computer logs you in by itself, and then stays open. Locking the screen doesn't
close it. So while you are logged in, an administrator of this computer, and any program
running as you outside a sandbox, can get to that key and read what it protects.

Project B also asks nothing more when a password is shown or a passkey is used on Linux:
your login is the check.

## Why a Mac and a phone are different

On a Mac, the key is in your login keychain. It opens only with your login password and
hands the key to Project B alone: another administrator's account can't read it without
your password, and other apps, even ones you run, are asked for your password before they
can. And macOS checks it's you, with Touch ID or your password, each time a passkey is used
or a password shown.

On a phone, the key never leaves Android's keystore, and your screen lock or fingerprint is
asked each time.

Linux desktops have no standard check that an app can ask for, and their keyring gives its
secrets to any program running as you that asks. This is how saved passwords work in other
browsers on Linux too.

## What you can do

- Use this computer's account only yourself, and lock the screen when you leave it.
- On a computer that others administer, or that you don't trust, keep passkeys on a
  security key or your phone: when a site asks for a passkey, choose "Use a security key or
  phone".
- Turning sync on doesn't change this: the sync passphrase protects what is on the sync
  server, not what is on this computer.

A lock for the profile, with a passphrase of its own that keeps saved data closed until you
type it, may come later.
