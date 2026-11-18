#ifndef PROJECT_B_ENGINE_BROWSER_PASSKEYS_H_
#define PROJECT_B_ENGINE_BROWSER_PASSKEYS_H_

#include <cstdint>
#include <string>
#include <vector>

#include "base/functional/callback.h"
#include "base/memory/weak_ptr.h"
#include "base/values.h"
#include "content/public/browser/authenticator_request_client_delegate.h"
#include "content/public/browser/global_routing_id.h"
#include "content/public/browser/web_authentication_delegate.h"
#include "device/fido/cable/v2_constants.h"

namespace content {
class RenderFrameHost;
}  // namespace content

namespace project_b {

// One passkey request of a page, navigator.credentials.create or .get with publicKey, on
// the Mac (docs/passkeys.md). Content makes one per request and drops it when the request
// ends. Go owns what the person sees: it hears of the request as a dialog of kind
// PB_DIALOG_PASSKEY, of each thing the person must do as PB_EVENT_PASSKEY_STEP, and of its
// end as PB_EVENT_DIALOG_CLOSED; what the person did comes back through Answer.
//
// A request with no card of its own (a sign-in field's passkey list, a test's virtual key
// with its UI off) tells Go nothing and goes as content's own delegate would.
class PasskeyRequest : public content::AuthenticatorRequestClientDelegate {
 public:
  explicit PasskeyRequest(content::RenderFrameHost* frame);
  ~PasskeyRequest() override;

  // Go's answer_passkey: what the person did at request id's step.
  static void Answer(int64_t id, const std::string& action, const std::string& text);

  // content::AuthenticatorRequestClientDelegate:
  void SetRelyingPartyId(const std::string& rp_id) override;
  void SetUIPresentation(UIPresentation ui_presentation) override;
  bool DoesBlockRequestOnFailure(InterestingFailureReason reason) override;
  void RegisterActionCallbacks(
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
          request_ble_permission_callback) override;
  void ConfigureDiscoveries(
      const url::Origin& origin,
      const std::string& rp_id,
      RequestSource request_source,
      device::FidoRequestType request_type,
      std::optional<device::ResidentKeyRequirement> resident_key_requirement,
      device::UserVerificationRequirement user_verification_requirement,
      bool cmtg_key_requested,
      std::optional<std::string_view> user_name,
      bool is_enclave_authenticator_available,
      device::FidoDiscoveryFactory* fido_discovery_factory) override;
  void SelectAccount(
      std::vector<device::AuthenticatorGetAssertionResponse> responses,
      base::OnceCallback<void(device::AuthenticatorGetAssertionResponse)>
          callback) override;
  void OnTransportAvailabilityEnumerated(
      device::FidoRequestHandlerBase::TransportAvailabilityInfo data) override;
  void BluetoothAdapterStatusChanged(
      device::FidoRequestHandlerBase::BleStatus ble_status) override;
  bool SupportsPIN() const override;
  void CollectPIN(CollectPINOptions options,
                  base::OnceCallback<void(std::u16string)> provide_pin_cb) override;
  void StartBioEnrollment(base::OnceClosure next_callback) override;
  void FinishCollectToken() override;
  void OnRetryUserVerification(int attempts) override;

 private:
  void Answered(const std::string& action, const std::string& text);
  // Tells Go of a step, and of the request first when this is its first.
  void Step(const std::string& name, base::DictValue carries);
  void KeyStep();
  void PhoneStep();
  void OnBluetoothPermission(device::FidoRequestHandlerBase::BleStatus status);
  void OnPhoneEvent(device::cablev2::Event event);

  const content::GlobalRenderFrameHostId frame_;
  // Go's id for the request, 0 until Go is told of it.
  int64_t id_ = 0;
  int64_t tab_ = 0;
  // Whether the request has a card: content asked for its own modal UI, in a tab.
  bool shown_ = false;
  std::string rp_id_;
  device::FidoRequestType type_ = device::FidoRequestType::kGetAssertion;

  // Phone as key: whether it is there, the link its QR code carries, what Bluetooth is
  // doing, whether the person is at the phone step, and whether a phone has the request.
  bool phone_ = false;
  std::string link_;
  device::FidoRequestHandlerBase::BleStatus bluetooth_ =
      device::FidoRequestHandlerBase::BleStatus::kOff;
  bool can_turn_on_ = false;
  bool at_phone_ = false;
  bool phone_busy_ = false;
  bool phone_ready_ = false;

  base::OnceClosure cancel_;
  base::RepeatingClosure start_over_;
  base::RepeatingClosure bluetooth_on_;
  base::RepeatingCallback<void(device::FidoRequestHandlerBase::BlePermissionCallback)>
      ask_bluetooth_;
  base::OnceCallback<void(std::u16string)> pin_;
  std::vector<device::AuthenticatorGetAssertionResponse> accounts_;
  base::OnceCallback<void(device::AuthenticatorGetAssertionResponse)> account_;

  base::WeakPtrFactory<PasskeyRequest> weak_factory_{this};
};

// What content asks about passkeys outside one request. Keys that remember their
// accounts are allowed, as the account step chooses between them.
class PasskeyPolicy : public content::WebAuthenticationDelegate {
 public:
  bool SupportsResidentKeys(content::RenderFrameHost* render_frame_host) override;
};

}  // namespace project_b

#endif  // PROJECT_B_ENGINE_BROWSER_PASSKEYS_H_
