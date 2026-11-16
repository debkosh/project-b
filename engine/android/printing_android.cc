// A phone's part of printing: Android's print sheet, which prints the PDF the page became
// or saves it as a PDF, with its own preview.

#include <algorithm>
#include <string>

#include "base/android/jni_android.h"
#include "base/android/jni_array.h"
#include "base/android/jni_string.h"
#include "base/containers/span.h"
#include "base/i18n/rtl.h"
#include "content/public/browser/web_contents.h"
#include "printing/units.h"
#include "project_b/engine/android/jni_headers/Printing_jni.h"
#include "project_b/engine/browser/printing.h"

namespace project_b {
namespace printing {

// Android's sheet chooses the paper after the page is drawn and fits it to what it chose;
// the page is drawn on the locale's usual paper: Letter in the Americas that use it, A4
// everywhere else.
gfx::SizeF PaperInches() {
  const std::string locale = base::i18n::GetConfiguredLocale();
  const size_t dash = locale.find_last_of("-_");
  const std::string country =
      dash == std::string::npos ? "" : locale.substr(dash + 1);
  static constexpr const char* kLetter[] = {"US", "CA", "MX", "PH", "CL",
                                            "CO", "VE", "PR", "GT", "CR"};
  if (std::ranges::find(kLetter, country) != std::end(kLetter)) {
    return gfx::SizeF(::printing::kLetterWidthInch, ::printing::kLetterHeightInch);
  }
  return gfx::SizeF(::printing::kA4WidthInch, ::printing::kA4HeightInch);
}

void ShowPanel(content::WebContents* contents,
               const std::u16string& title,
               scoped_refptr<base::RefCountedMemory> pdf) {
  JNIEnv* env = base::android::AttachCurrentThread();
  Java_Printing_print(env, base::android::ConvertUTF16ToJavaString(env, title),
                      base::android::ToJavaByteArray(env, base::span(*pdf)));
}

}  // namespace printing
}  // namespace project_b
