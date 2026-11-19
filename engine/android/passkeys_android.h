#ifndef PROJECT_B_ENGINE_ANDROID_PASSKEYS_ANDROID_H_
#define PROJECT_B_ENGINE_ANDROID_PASSKEYS_ANDROID_H_

#include <cstdint>
#include <string>

namespace project_b {

// Go's answer_passkey on a phone: what the person did at request id's step, or Go's answer
// to our own passkeys, to the frame's authenticator in Java (Passkeys.java).
void AnswerPasskey(int64_t id, const std::string& action, const std::string& text);

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_ANDROID_PASSKEYS_ANDROID_H_
