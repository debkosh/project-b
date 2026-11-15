package dev.projectb.browser

import android.annotation.SuppressLint
import android.content.Context
import android.graphics.Typeface
import android.text.Editable
import android.text.InputType
import android.text.SpannableStringBuilder
import android.text.Spanned
import android.text.TextWatcher
import android.text.style.StyleSpan
import android.util.TypedValue
import android.view.Gravity
import android.view.KeyEvent
import android.view.View
import android.view.inputmethod.EditorInfo
import android.view.inputmethod.InputMethodManager
import android.widget.EditText
import android.widget.FrameLayout
import android.widget.LinearLayout
import android.widget.TextView
import org.json.JSONObject

// The native Mobile shell's daily-driver parts (docs/daily-driver.md), each the web Mobile
// shell's: the find bar, a page's dialog, and No tabs. Downloads is a page of ours, in a tab.

/** A text button of the design system: 48dp high, its words in the accent, or white on it. */
fun textButton(context: Context, palette: Palette, words: String, primary: Boolean = false, color: Int? = null, run: () -> Unit) =
    label(context, 14f, if (primary) palette.onAccent else (color ?: palette.accent), bold = true).apply {
        text = words
        gravity = Gravity.CENTER
        setPadding(dp(16), 0, dp(16), 0)
        isClickable = true
        setOnClickListener { run() }
        background = if (primary) rounded(palette.accent, dpf(8)) else android.graphics.drawable.StateListDrawable().apply {
            addState(intArrayOf(android.R.attr.state_pressed), rounded(palette.press, dpf(8)))
        }
        asButton(this, words)
    }

private fun field(context: Context, palette: Palette, name: String) = EditText(context).apply {
    setSingleLine(true)
    setTextSize(TypedValue.COMPLEX_UNIT_DIP, 14f)
    setTextColor(palette.text)
    setHintTextColor(palette.muted)
    background = rounded(palette.surface, dpf(8), palette.line, dp(1))
    setPadding(dp(12), 0, dp(12), 0)
    contentDescription = name
}

/**
 * Find in page, in the toolbar's place while the shown tab's search is open: what is typed
 * searches as it changes, the count at the field's end; the arrows and the keyboard's Enter go
 * to the match after or before; × closes it, as the phone's back key does.
 */
@SuppressLint("ViewConstructor")
class FindBar(context: Context, private val send: (JSONObject) -> Unit, private val closed: () -> Unit) : Bar(context) {
    private val box = FrameLayout(context)
    val input = EditText(context)
    private val count = TextView(context)
    private val up = IconButton(context, "up", "Previous match")
    private val down = IconButton(context, "down", "Next match")
    private val close = IconButton(context, "close", "Close find")
    private var tab = 0L
    // The tab whose text the field shows: it follows Go's only when another tab's search shows.
    private var shownTab = 0L

    init {
        setPadding(dp(8), 1, 0, 0)
        input.apply {
            background = null
            setSingleLine(true)
            setTextSize(TypedValue.COMPLEX_UNIT_DIP, 14f)
            setPadding(dp(14), 0, dp(56), 0)
            hint = "Find in page"
            contentDescription = "Find in page"
            inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
            imeOptions = EditorInfo.IME_ACTION_SEARCH or EditorInfo.IME_FLAG_NO_EXTRACT_UI
            addTextChangedListener(object : TextWatcher {
                override fun beforeTextChanged(s: CharSequence?, a: Int, b: Int, c: Int) {}
                override fun onTextChanged(s: CharSequence?, a: Int, b: Int, c: Int) {}
                override fun afterTextChanged(s: Editable?) {
                    if (hasFocus()) send(JSONObject().put("t", "findText").put("tab", tab).put("text", text.toString()))
                }
            })
            setOnEditorActionListener { _, action, event ->
                val enter = event?.keyCode == KeyEvent.KEYCODE_ENTER && event.action == KeyEvent.ACTION_DOWN
                if (action == EditorInfo.IME_ACTION_SEARCH || enter) { next(true); true } else false
            }
        }
        count.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 12f)
        count.gravity = Gravity.CENTER_VERTICAL or Gravity.END
        count.setPadding(0, 0, dp(14), 0)
        box.addView(input, FrameLayout.LayoutParams(FrameLayout.LayoutParams.MATCH_PARENT, FrameLayout.LayoutParams.MATCH_PARENT))
        box.addView(count, FrameLayout.LayoutParams(FrameLayout.LayoutParams.WRAP_CONTENT, FrameLayout.LayoutParams.MATCH_PARENT, Gravity.END))
        addView(box, LayoutParams(0, dp(48), 1f).apply { marginEnd = dp(4) })
        up.setOnClickListener { next(false) }
        down.setOnClickListener { next(true) }
        close.setOnClickListener { close() }
        for (b in listOf(up, down, close)) addView(b, LayoutParams(dp(48), dp(48)))
    }

    private fun next(forward: Boolean) = send(JSONObject().put("t", "findNext").put("tab", tab).put("forward", forward))

    fun close() {
        send(JSONObject().put("t", "findClose").put("tab", tab))
        closed()
    }

    /** Go's search for the shown tab: its text, and the count as "active/matches". */
    fun show(palette: Palette, find: JSONObject) {
        tab = find.optLong("tab")
        setBackgroundColor(palette.bar)
        line = palette.line
        box.background = android.graphics.drawable.InsetDrawable(rounded(palette.surface, dpf(20)), 0, dp(4), 0, dp(4))
        box.setPadding(0, 0, 0, 0)
        input.setTextColor(palette.text)
        input.setHintTextColor(palette.muted)
        if (tab != shownTab) {
            shownTab = tab
            input.setText(find.optString("text"))
        }
        val text = find.optString("text")
        val matches = find.optInt("matches")
        count.text = if (text.isEmpty()) "" else if (matches > 0) "${find.optInt("active")}/$matches" else "0/0"
        count.setTextColor(if (text.isNotEmpty() && matches == 0) palette.danger else palette.muted)
        for (b in listOf(up, down, close)) { b.ink = palette.icon; b.press = palette.press }
        up.isEnabled = matches > 0
        down.isEnabled = matches > 0
    }

    /** focusFind's answer: the keyboard in the field, its text selected. */
    fun focus() {
        send(JSONObject().put("t", "focusShell"))
        post {
            input.requestFocus()
            input.selectAll()
            (context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager).showSoftInput(input, 0)
        }
    }
}

/**
 * A page's alert, confirm, prompt, leave-page question or sign-in, in the middle of the screen
 * over the dimmed window, as a phone's dialogs are, fading in. Go words it; the page waits until
 * it is answered. The phone's back key answers no.
 */
@SuppressLint("ViewConstructor")
class PageDialog(context: Context, palette: Palette, private val dialog: JSONObject, private val send: (JSONObject) -> Unit,
                 private val answered: () -> Unit) : FrameLayout(context) {
    private var first: EditText? = null
    private var text: EditText? = null
    private var password: EditText? = null

    init {
        setBackgroundColor(Palette.alpha(palette.bar, 0.7f))
        isClickable = true
        val card = LinearLayout(context).apply {
            orientation = LinearLayout.VERTICAL
            setPadding(dp(24), dp(20), dp(16), dp(12))
            background = rounded(palette.surface, dpf(16))
            elevation = dpf(8)
            isClickable = true
        }
        val title = dialog.optString("title")
        card.contentDescription = title
        card.addView(label(context, 16f, palette.text, bold = true).apply { this.text = title; setSingleLine(false) },
            LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { marginEnd = dp(8) })
        val message = dialog.optString("message")
        if (message.isNotEmpty()) {
            val host = dialog.optString("host")
            val words = SpannableStringBuilder(message)
            val at = if (host.isNotEmpty()) message.indexOf(host) else -1
            if (at >= 0) words.setSpan(StyleSpan(Typeface.BOLD), at, at + host.length, Spanned.SPAN_EXCLUSIVE_EXCLUSIVE)
            card.addView(label(context, 14f, palette.text).apply { this.text = words; setSingleLine(false); maxLines = 12 },
                LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { topMargin = dp(12); marginEnd = dp(8) })
        }
        fun place(v: View) = card.addView(v, LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(48)).apply { topMargin = dp(12); marginEnd = dp(8) })
        when (dialog.optString("kind")) {
            "prompt" -> text = field(context, palette, message.ifEmpty { "Answer" }).apply {
                setText(dialog.optString("defaultText"))
                imeOptions = EditorInfo.IME_ACTION_DONE or EditorInfo.IME_FLAG_NO_EXTRACT_UI
                setOnEditorActionListener { _, _, _ -> answer(true); true }
            }.also { place(it); first = it }
            "signin" -> {
                val realm = dialog.optString("realm")
                if (realm.isNotEmpty()) card.addView(label(context, 12f, palette.muted).apply { this.text = "The site says: “$realm”"; setSingleLine(false) },
                    LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { topMargin = dp(8) })
                card.addView(label(context, 12f, palette.muted).apply { this.text = "Username" },
                    LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { topMargin = dp(12) })
                text = field(context, palette, "Username").apply {
                    inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_FLAG_NO_SUGGESTIONS
                    imeOptions = EditorInfo.IME_ACTION_NEXT or EditorInfo.IME_FLAG_NO_EXTRACT_UI
                }.also { card.addView(it, LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(48)).apply { topMargin = dp(4); marginEnd = dp(8) }); first = it }
                card.addView(label(context, 12f, palette.muted).apply { this.text = "Password" },
                    LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { topMargin = dp(12) })
                password = field(context, palette, "Password").apply {
                    inputType = InputType.TYPE_CLASS_TEXT or InputType.TYPE_TEXT_VARIATION_PASSWORD
                    imeOptions = EditorInfo.IME_ACTION_DONE or EditorInfo.IME_FLAG_NO_EXTRACT_UI
                    setOnEditorActionListener { _, _, _ -> answer(true); true }
                }.also { card.addView(it, LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, dp(48)).apply { topMargin = dp(4); marginEnd = dp(8) }) }
                if (dialog.optBoolean("insecure")) card.addView(label(context, 12f, palette.danger).apply {
                    this.text = "This connection isn't secure: your password will be sent as plain text."; setSingleLine(false)
                }, LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { topMargin = dp(8) })
            }
        }
        val buttons = LinearLayout(context).apply { orientation = LinearLayout.HORIZONTAL; gravity = Gravity.END }
        val cancel = dialog.optString("cancel")
        if (cancel.isNotEmpty()) buttons.addView(textButton(context, palette, cancel) { answer(false) },
            LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, dp(48)).apply { marginEnd = dp(8) })
        buttons.addView(textButton(context, palette, dialog.optString("ok", "OK"), primary = true) { answer(true) },
            LinearLayout.LayoutParams(LinearLayout.LayoutParams.WRAP_CONTENT, dp(48)))
        card.addView(buttons, LinearLayout.LayoutParams(LinearLayout.LayoutParams.MATCH_PARENT, LinearLayout.LayoutParams.WRAP_CONTENT).apply { topMargin = dp(20) })
        addView(card, LayoutParams(LayoutParams.MATCH_PARENT, LayoutParams.WRAP_CONTENT, Gravity.CENTER).apply { marginStart = dp(24); marginEnd = dp(24) })
        // A dialog says so to a screen reader and the driver, as the web's role=alertdialog.
        card.accessibilityDelegate = object : AccessibilityDelegate() {
            override fun onInitializeAccessibilityNodeInfo(host: View, info: android.view.accessibility.AccessibilityNodeInfo) {
                super.onInitializeAccessibilityNodeInfo(host, info)
                info.className = "android.app.AlertDialog"
            }
        }
    }

    override fun onAttachedToWindow() {
        super.onAttachedToWindow()
        // It fades in, never grows, so its 48dp targets are 48dp from the first frame.
        alpha = 0f
        animate().alpha(1f).setDuration(Motion.ms(context)).setInterpolator(Motion.ease).start()
        send(JSONObject().put("t", "focusShell"))
        first?.let { f ->
            f.post {
                f.requestFocus()
                (context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager).showSoftInput(f, 0)
            }
        }
    }

    fun answer(yes: Boolean) {
        send(JSONObject().put("t", "answerDialog").put("id", dialog.opt("id")).put("ok", yes)
            .put("text", if (yes) text?.text?.toString() ?: "" else "")
            .put("password", if (yes) password?.text?.toString() ?: "" else ""))
        (context.getSystemService(Context.INPUT_METHOD_SERVICE) as InputMethodManager).hideSoftInputFromWindow(windowToken, 0)
        answered()
    }
}

/** A window with no tabs, as the back key leaves one before the app goes: No tabs, and New tab. */
@SuppressLint("ViewConstructor")
class NoTabs(context: Context, private val newTab: () -> Unit) : LinearLayout(context) {
    private val words = TextView(context)
    private var button: View? = null

    init {
        orientation = VERTICAL
        gravity = Gravity.CENTER
        isClickable = true
        words.text = "No tabs"
        words.setTextSize(TypedValue.COMPLEX_UNIT_DIP, 16f)
        addView(words, LayoutParams(LayoutParams.WRAP_CONTENT, LayoutParams.WRAP_CONTENT))
    }

    fun show(palette: Palette) {
        setBackgroundColor(palette.bar)
        words.setTextColor(palette.text)
        button?.let { removeView(it) }
        button = textButton(context, palette, "New tab", primary = true) { newTab() }.also {
            asButton(it, "New Tab")
            addView(it, LayoutParams(LayoutParams.WRAP_CONTENT, dp(48)).apply { topMargin = dp(16) })
        }
    }
}
