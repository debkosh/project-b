#ifndef PROJECT_B_ENGINE_COMMON_WORLDS_H_
#define PROJECT_B_ENGINE_COMMON_WORLDS_H_

#include "content/public/common/isolated_world_ids.h"

namespace project_b {

// The isolated worlds of a page that are ours, from the first one content leaves free.
// The form script's is one world, the same in every frame (docs/password-autofill.md);
// extensions' content scripts get theirs from the next one up.
inline constexpr int kFormsWorld = content::ISOLATED_WORLD_ID_CONTENT_END + 1;
inline constexpr int kFirstExtensionWorld = kFormsWorld + 1;

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_COMMON_WORLDS_H_
