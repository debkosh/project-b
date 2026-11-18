#include "project_b/engine/browser/passkeys.h"

#include <array>
#include <map>
#include <utility>

#include "base/functional/bind.h"
#include "base/json/json_writer.h"
#include "base/mac/mac_util.h"
#include "base/no_destructor.h"
#include "base/strings/string_number_conversions.h"
#include "base/strings/utf_string_conversions.h"
#include "base/task/single_thread_task_runner.h"
#include "content/public/browser/render_frame_host.h"
#include "content/public/browser/storage_partition.h"
#include "content/public/browser/web_contents.h"
#include "crypto/random.h"
#include "device/fido/cable/v2_handshake.h"
#include "device/fido/fido_discovery_factory.h"
#include "device/fido/pin.h"
#include "device/fido/public/fido_transport_protocol.h"
#include "project_b/engine/abi/pb_abi.h"
#include "project_b/engine/browser/dialogs.h"
#include "project_b/engine/browser/hooks.h"
#include "project_b/engine/browser/timeline.h"
#include "url/origin.h"

namespace project_b {

namespace {

using BleStatus = device::FidoRequestHandlerBase::BleStatus;
using Reason = content::AuthenticatorRequestClientDelegate::InterestingFailureReason;

// The requests Go knows of, by its id.
std::map<int64_t, PasskeyRequest*>& Requests() {
  static base::NoDestructor<std::map<int64_t, PasskeyRequest*>> requests;
  return *requests;
}

// A failure's name for Go, or null for one the page hears at once: the person said no, or
// it is not ours to show.
const char* NameOf(Reason reason) {
  switch (reason) {
    case Reason::kTimeout:
      return "timeout";
    case Reason::kKeyNotRegistered:
      return "not-registered";
    case Reason::kKeyAlreadyRegistered:
      return "already-registered";
    case Reason::kSoftPINBlock:
      return "pin-soft-blocked";
    case Reason::kHardPINBlock:
      return "pin-hard-blocked";
    case Reason::kAuthenticatorRemovedDuringPINEntry:
      return "key-removed";
    case Reason::kAuthenticatorMissingResidentKeys:
      return "no-passkey-support";
    case Reason::kAuthenticatorMissingUserVerification:
      return "no-uv";
    case Reason::kAuthenticatorMissingLargeBlob:
      return "no-large-blob";
    case Reason::kNoCommonAlgorithms:
      return "no-algorithm";
    case Reason::kStorageFull:
      return "full";
    case Reason::kHybridTransportError:
      return "phone-failed";
    case Reason::kUserConsentDenied:
    case Reason::kWinUserCancelled:
    case Reason::kNoPasskeys:
    case Reason::kEnclaveError:
    case Reason::kEnclaveCancel:
      return nullptr;
  }
  return "other";
}

const char* NameOf(device::pin::PINEntryReason reason) {
  switch (reason) {
    case device::pin::PINEntryReason::kSet:
      return "set";
    case device::pin::PINEntryReason::kChange:
      return "change";
    case device::pin::PINEntryReason::kChallenge:
      return "enter";
  }
  return "enter";
}

const char* NameOf(device::pin::PINEntryError error) {
  switch (error) {
    case device::pin::PINEntryError::kNoError:
      return "";
    case device::pin::PINEntryError::kInternalUvLocked:
      return "uv-locked";
    case device::pin::PINEntryError::kWrongPIN:
      return "wrong";
    case device::pin::PINEntryError::kTooShort:
      return "short";
    case device::pin::PINEntryError::kInvalidCharacters:
      return "invalid";
    case device::pin::PINEntryError::kSameAsCurrentPIN:
      return "same";
  }
  return "";
}

const char* NameOf(BleStatus status) {
  switch (status) {
    case BleStatus::kOn:
      return "on";
    case BleStatus::kPermissionDenied:
      return "denied";
    case BleStatus::kOff:
    case BleStatus::kPendingPermissionRequest:
      return "off";
  }
  return "off";
}

}  // namespace

PasskeyRequest::PasskeyRequest(content::RenderFrameHost* frame)
    : frame_(frame->GetGlobalId()),
      tab_(hooks::TabOf(content::WebContents::FromRenderFrameHost(frame))) {}

PasskeyRequest::~PasskeyRequest() {
  if (id_) {
    Requests().erase(id_);
    hooks::Args args;
    args.Int(id_);
    hooks::Fire(PB_EVENT_DIALOG_CLOSED, args);
  }
}

// static
void PasskeyRequest::Answer(int64_t id, const std::string& action, const std::string& text) {
  auto it = Requests().find(id);
  if (it != Requests().end()) {
    it->second->Answered(action, text);
  }
}

void PasskeyRequest::Answered(const std::string& action, const std::string& text) {
  if (action == "cancel") {
    // Content ends the request, and with it this.
    if (cancel_) {
      std::move(cancel_).Run();
    }
  } else if (action == "pin") {
    if (pin_) {
      std::move(pin_).Run(base::UTF8ToUTF16(text));
    }
  } else if (action == "account") {
    size_t index = 0;
    if (account_ && base::StringToSizeT(text, &index) && index < accounts_.size()) {
      device::AuthenticatorGetAssertionResponse chosen = std::move(accounts_[index]);
      accounts_.clear();
      std::move(account_).Run(std::move(chosen));
    }
  } else if (action == "phone") {
    if (!phone_) {
      return;
    }
    at_phone_ = true;
    if (bluetooth_ == BleStatus::kPendingPermissionRequest && ask_bluetooth_) {
      // macOS asks the person, the first time; the step follows its answer.
      ask_bluetooth_.Run(base::BindOnce(&PasskeyRequest::OnBluetoothPermission,
                                        weak_factory_.GetWeakPtr()));
      return;
    }
    PhoneStep();
  } else if (action == "key") {
    at_phone_ = false;
    KeyStep();
  } else if (action == "bluetoothOn") {
    if (bluetooth_on_) {
      bluetooth_on_.Run();
    }
  } else if (action == "bluetoothSettings") {
    base::mac::OpenSystemSettingsPane(
        base::mac::SystemSettingsPane::kPrivacySecurity_Bluetooth);
  } else if (action == "retry") {
    // Content starts the request again from its discoveries; the key step follows.
    pin_.Reset();
    account_.Reset();
    accounts_.clear();
    at_phone_ = phone_busy_ = phone_ready_ = false;
    if (start_over_) {
      start_over_.Run();
    }
  }
}

void PasskeyRequest::Step(const std::string& name, base::DictValue carries) {
  if (!shown_) {
    return;
  }
  if (!id_) {
    id_ = Dialogs::Get()->NextId();
    Requests()[id_] = this;
    content::RenderFrameHost* frame = content::RenderFrameHost::FromID(frame_);
    hooks::Args args;
    args.Int(id_);
    args.Int(tab_);
    args.Int(PB_DIALOG_PASSKEY);
    args.Str(rp_id_);
    args.Str(std::string());
    args.Str(frame ? frame->GetLastCommittedOrigin().Serialize() : std::string());
    args.Bool(false);
    hooks::Fire(PB_EVENT_DIALOG_OPENED, args);
  }
  // The step's name only: what it carries can hold the phone's link.
  timeline::Mark("passkey.step", "dialog=" + base::NumberToString(id_) + " step=" + name);
  hooks::Args args;
  args.Int(id_);
  args.Str(name);
  args.Str(base::WriteJson(carries).value_or("{}"));
  hooks::Fire(PB_EVENT_PASSKEY_STEP, args);
}

void PasskeyRequest::KeyStep() {
  base::DictValue carries;
  carries.Set("request", type_ == device::FidoRequestType::kMakeCredential ? "create" : "get");
  carries.Set("site", rp_id_);
  carries.Set("phone", phone_);
  Step("key", std::move(carries));
}

void PasskeyRequest::PhoneStep() {
  base::DictValue carries;
  carries.Set("link", link_);
  carries.Set("bluetooth", NameOf(bluetooth_));
  carries.Set("canTurnOn", bluetooth_ == BleStatus::kOff && can_turn_on_);
  Step("phone", std::move(carries));
}

void PasskeyRequest::OnBluetoothPermission(BleStatus status) {
  bluetooth_ = status;
  if (at_phone_ && !phone_busy_) {
    PhoneStep();
  }
}

void PasskeyRequest::OnPhoneEvent(device::cablev2::Event event) {
  if (event == device::cablev2::Event::kReady) {
    phone_ready_ = true;
  }
  if (!phone_busy_) {
    phone_busy_ = true;
    Step("phoneBusy", base::DictValue());
  }
}

void PasskeyRequest::SetRelyingPartyId(const std::string& rp_id) {
  rp_id_ = rp_id;
}

void PasskeyRequest::SetUIPresentation(UIPresentation ui_presentation) {
  shown_ = ui_presentation == UIPresentation::kModal && tab_ > 0;
  if (!shown_) {
    timeline::Mark("passkey.quiet",
                   "presentation=" + base::NumberToString(static_cast<int>(ui_presentation)) +
                       " tab=" + base::NumberToString(tab_));
  }
}

bool PasskeyRequest::DoesBlockRequestOnFailure(InterestingFailureReason reason) {
  // Once a phone has the request, its end is the phone's to show: the person most likely
  // said no there.
  if (!shown_ || !id_ || phone_ready_) {
    return false;
  }
  const char* name = NameOf(reason);
  if (!name) {
    return false;
  }
  base::DictValue carries;
  carries.Set("reason", name);
  Step("error", std::move(carries));
  return true;
}

void PasskeyRequest::RegisterActionCallbacks(
    base::OnceClosure cancel_callback,
    base::OnceClosure immediate_not_found_callback,
    base::RepeatingClosure start_over_callback,
    AccountPreselectedCallback account_preselected_callback,
    PasswordSelectedCallback password_selected_callback,
    device::FidoRequestHandlerBase::RequestCallback request_callback,
    base::OnceClosure cancel_ui_timeout_callback,
    base::RepeatingClosure bluetooth_adapter_power_on_callback,
    base::RepeatingCallback<
        void(device::FidoRequestHandlerBase::BlePermissionCallback)>
        request_ble_permission_callback) {
  cancel_ = std::move(cancel_callback);
  start_over_ = std::move(start_over_callback);
  bluetooth_on_ = std::move(bluetooth_adapter_power_on_callback);
  ask_bluetooth_ = std::move(request_ble_permission_callback);
}

void PasskeyRequest::ConfigureDiscoveries(
    const url::Origin& origin,
    const std::string& rp_id,
    RequestSource request_source,
    device::FidoRequestType request_type,
    std::optional<device::ResidentKeyRequirement> resident_key_requirement,
    device::UserVerificationRequirement user_verification_requirement,
    bool cmtg_key_requested,
    std::optional<std::string_view> user_name,
    bool is_enclave_authenticator_available,
    device::FidoDiscoveryFactory* fido_discovery_factory) {
  if (!shown_ || !fido_discovery_factory) {
    return;
  }
  // Phone as key: a new secret for each request's QR code, so no phone is remembered; the
  // tab's own network for the tunnel; and word of the phone taking the request.
  std::array<uint8_t, device::cablev2::kQRKeySize> key;
  crypto::RandBytes(key);
  link_ = device::cablev2::qr::Encode(key, request_type);
  fido_discovery_factory->set_cable_data(request_type, key);
  fido_discovery_factory->set_network_context_factory(base::BindRepeating(
      [](content::GlobalRenderFrameHostId id) -> network::mojom::NetworkContext* {
        content::RenderFrameHost* frame = content::RenderFrameHost::FromID(id);
        return frame ? frame->GetStoragePartition()->GetNetworkContext() : nullptr;
      },
      frame_));
  fido_discovery_factory->set_cable_event_callback(base::BindRepeating(
      &PasskeyRequest::OnPhoneEvent, weak_factory_.GetWeakPtr()));
}

void PasskeyRequest::SelectAccount(
    std::vector<device::AuthenticatorGetAssertionResponse> responses,
    base::OnceCallback<void(device::AuthenticatorGetAssertionResponse)> callback) {
  // With no card, or one account, there is nothing to choose.
  if (!shown_ || responses.size() == 1) {
    std::move(callback).Run(std::move(responses.at(0)));
    return;
  }
  base::ListValue accounts;
  for (const auto& response : responses) {
    base::DictValue account;
    if (response.user_entity) {
      account.Set("name", response.user_entity->name.value_or(std::string()));
      account.Set("displayName", response.user_entity->display_name.value_or(std::string()));
    }
    accounts.Append(std::move(account));
  }
  accounts_ = std::move(responses);
  account_ = std::move(callback);
  base::DictValue carries;
  carries.Set("accounts", std::move(accounts));
  Step("account", std::move(carries));
}

void PasskeyRequest::StartObserving(device::FidoRequestHandlerBase* request_handler) {
  // A request with no card goes as content's own delegate's does, which observes nothing.
  if (shown_) {
    handler_.Observe(request_handler);
  }
}

void PasskeyRequest::StopObserving(device::FidoRequestHandlerBase* request_handler) {
  handler_.Reset();
}

void PasskeyRequest::OnTransportAvailabilityEnumerated(
    device::FidoRequestHandlerBase::TransportAvailabilityInfo data) {
  type_ = data.request_type;
  phone_ = !link_.empty() &&
           data.available_transports.contains(device::FidoTransportProtocol::kHybrid);
  bluetooth_ = data.ble_status;
  can_turn_on_ = data.can_power_on_ble_adapter;
  KeyStep();
}

void PasskeyRequest::BluetoothAdapterStatusChanged(BleStatus ble_status) {
  bluetooth_ = ble_status;
  if (at_phone_ && !phone_busy_) {
    PhoneStep();
  }
}

bool PasskeyRequest::SupportsPIN() const {
  return shown_;
}

void PasskeyRequest::CollectPIN(CollectPINOptions options,
                                base::OnceCallback<void(std::u16string)> provide_pin_cb) {
  pin_ = std::move(provide_pin_cb);
  base::DictValue carries;
  carries.Set("reason", NameOf(options.reason));
  carries.Set("error", NameOf(options.error));
  carries.Set("attempts", options.attempts);
  carries.Set("minLength", static_cast<int>(options.min_pin_length));
  Step("pin", std::move(carries));
}

void PasskeyRequest::StartBioEnrollment(base::OnceClosure next_callback) {
  // Adding a fingerprint to a key is its maker's app's work; the request goes on without.
  base::SingleThreadTaskRunner::GetCurrentDefault()->PostTask(FROM_HERE,
                                                              std::move(next_callback));
}

void PasskeyRequest::FinishCollectToken() {
  Step("touch", base::DictValue());
}

void PasskeyRequest::OnRetryUserVerification(int attempts) {
  base::DictValue carries;
  carries.Set("attempts", attempts);
  Step("touch", std::move(carries));
}

bool PasskeyPolicy::SupportsResidentKeys(content::RenderFrameHost* render_frame_host) {
  return true;
}

}  // namespace project_b
