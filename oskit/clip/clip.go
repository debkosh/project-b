// Package clip is the clipboard's secret copy: a password a person copied, marked so that what
// keeps or shows the clipboard's history leaves it out. The Mac and Android have halves; on
// Linux no portal marks a secret, and the app writes the clipboard itself.
package clip

// Clipboard is the OS's clipboard, for a secret.
type Clipboard interface {
	// CopySecret puts text on the clipboard, marked a secret: on the Mac the pasteboard's
	// concealed and transient types beside it, which clipboard managers and Universal
	// Clipboard's history honour; on Android the clip's sensitive flag, which also hides the
	// system's preview of it from Android 13.
	CopySecret(text string)
}
