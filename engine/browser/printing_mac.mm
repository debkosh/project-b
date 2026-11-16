// The Mac's part of printing: the paper the default printer uses, and the system's print
// panel as a sheet on the tab's window, its preview the PDF the page became.

#import <AppKit/AppKit.h>
#import <PDFKit/PDFKit.h>

#include "base/strings/sys_string_conversions.h"
#include "content/public/browser/web_contents.h"
#include "project_b/engine/browser/printing.h"
#include "ui/gfx/native_ui_types.h"

namespace project_b {
namespace printing {

gfx::SizeF PaperInches() {
  // In points, 72 to the inch.
  NSSize paper = NSPrintInfo.sharedPrintInfo.paperSize;
  if (paper.width <= 0 || paper.height <= 0) {
    return gfx::SizeF(8.5, 11);
  }
  return gfx::SizeF(paper.width / 72, paper.height / 72);
}

void ShowPanel(content::WebContents* contents,
               const std::u16string& title,
               scoped_refptr<base::RefCountedMemory> pdf) {
  NSData* data = [NSData dataWithBytes:pdf->front() length:pdf->size()];
  PDFDocument* document = [[PDFDocument alloc] initWithData:data];
  if (!document) {
    return;
  }
  NSPrintInfo* info = [NSPrintInfo.sharedPrintInfo copy];
  // The PDF has its margins already.
  info.topMargin = info.bottomMargin = info.leftMargin = info.rightMargin = 0;
  NSPrintOperation* operation =
      [document printOperationForPrintInfo:info
                               scalingMode:kPDFPrintPageScaleDownToFit
                                autoRotate:YES];
  // Save as PDF in the panel names the file after the page.
  operation.jobTitle = base::SysUTF16ToNSString(title);
  operation.showsPrintPanel = YES;
  operation.showsProgressPanel = YES;
  operation.printPanel.options |= NSPrintPanelShowsPaperSize |
                                  NSPrintPanelShowsOrientation |
                                  NSPrintPanelShowsPreview;
  NSWindow* window = contents->GetTopLevelNativeWindow().GetNativeNSWindow();
  if (window) {
    [operation runOperationModalForWindow:window
                                 delegate:nil
                           didRunSelector:nil
                              contextInfo:nullptr];
  } else {
    [operation runOperation];
  }
}

}  // namespace printing
}  // namespace project_b
