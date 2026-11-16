#ifndef PROJECT_B_ENGINE_BROWSER_PRINTING_H_
#define PROJECT_B_ENGINE_BROWSER_PRINTING_H_

#include <string>

#include "base/memory/ref_counted_memory.h"
#include "base/memory/scoped_refptr.h"
#include "ui/gfx/geometry/size_f.h"

namespace blink {
class AssociatedInterfaceRegistry;
}

namespace content {
class RenderFrameHost;
class WebContents;
}  // namespace content

namespace project_b {
namespace printing {

// Printing (docs/daily-driver.md): Chromium's printing component renders a page to a PDF,
// and the system's print panel, which has its own preview, prints it. There is no preview
// of ours.

// Makes a tab printable: its print manager, which answers its frames' printing helpers.
void Follow(content::WebContents* contents);

// Takes a frame's window.print(): Go is asked, and the frame printed if it says so.
void BindFrame(content::RenderFrameHost& frame,
               blink::AssociatedInterfaceRegistry& registry);

// Prints a tab's page: renders it to a PDF on the paper the system would use, then shows
// the system's panel with it.
void Print(content::WebContents* contents);

// The platform's part: the paper a print goes on, in inches, as the system's default
// printer has it; and the system's panel over the tab's window, with the PDF in it.
gfx::SizeF PaperInches();
void ShowPanel(content::WebContents* contents,
               const std::u16string& title,
               scoped_refptr<base::RefCountedMemory> pdf);

}  // namespace printing
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_PRINTING_H_
