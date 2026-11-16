#ifndef PROJECT_B_ENGINE_COMMON_PRINT_HELPER_H_
#define PROJECT_B_ENGINE_COMMON_PRINT_HELPER_H_

namespace content {
class RenderFrame;
}

namespace project_b {
namespace printing {

// Makes a frame printable: Chromium's printing helper, which renders it to a PDF when the
// browser asks, and hands its window.print() to the browser (docs/daily-driver.md,
// Printing). Each frame, as the renderer makes it.
void Attach(content::RenderFrame* frame);

}  // namespace printing
}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_PRINT_HELPER_H_
