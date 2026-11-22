#ifndef PROJECT_B_ENGINE_BROWSER_FORMS_H_
#define PROJECT_B_ENGINE_BROWSER_FORMS_H_

#include <cstdint>
#include <string>

namespace content {
class RenderFrameHost;
class RenderProcessHost;
class WebContents;
}  // namespace content

namespace project_b::forms {

// The carrier of the page protocol (docs/password-autofill.md): the form script Go gives
// runs in every frame of every tab's page, in a world of its own, and its messages reach
// Go stamped with what the browser itself knows of the frame.

// Go's form script, given once; every renderer is told it, those to come too.
void SetScript(std::string source);
// A renderer is starting: it is told the script.
void RendererLaunching(content::RenderProcessHost* host);
// A frame's form script said kind and json: Go hears it as PB_EVENT_FORM_MESSAGE, with the
// frame as the browser knows it. Nothing for a frame of no tab, as the shell's pages are.
void FromFrame(content::RenderFrameHost* frame,
               const std::string& kind,
               const std::string& json);
// Sends kind and json to the form script of a frame of a tab's page, when the document
// there is still the one named. False when it isn't.
bool Send(content::WebContents* contents,
          const std::string& frame,
          const std::string& document,
          const std::string& kind,
          const std::string& json);

}  // namespace project_b::forms

#endif  // PROJECT_B_ENGINE_BROWSER_FORMS_H_
