package dev.projectb.browser;

import android.app.KeyguardManager;
import android.util.Base64;
import android.util.LongSparseArray;

import org.chromium.base.ContextUtils;
import org.chromium.blink.mojom.AuthenticationExtensionsClientOutputs;
import org.chromium.blink.mojom.Authenticator;
import org.chromium.blink.mojom.AuthenticatorAttachment;
import org.chromium.blink.mojom.AuthenticatorStatus;
import org.chromium.blink.mojom.AuthenticatorTransport;
import org.chromium.blink.mojom.CommonCredentialInfo;
import org.chromium.blink.mojom.GetAssertionAuthenticatorResponse;
import org.chromium.blink.mojom.GetAssertionResponse;
import org.chromium.blink.mojom.GetCredentialOptions;
import org.chromium.blink.mojom.GetCredentialResponse;
import org.chromium.blink.mojom.MakeCredentialAuthenticatorResponse;
import org.chromium.blink.mojom.Mediation;
import org.chromium.blink.mojom.PublicKeyCredentialCreationOptions;
import org.chromium.blink.mojom.PublicKeyCredentialDescriptor;
import org.chromium.blink.mojom.PublicKeyCredentialParameters;
import org.chromium.blink.mojom.PublicKeyCredentialReportOptions;
import org.chromium.blink.mojom.PublicKeyCredentialRequestOptions;
import org.chromium.blink.mojom.WebAuthnClientCapability;
import org.chromium.content_public.browser.ClientDataJson;
import org.chromium.content_public.browser.ClientDataRequestType;
import org.chromium.content_public.browser.InterfaceRegistrar;
import org.chromium.content_public.browser.RenderFrameHost;
import org.chromium.content_public.browser.WebContents;
import org.chromium.content_public.browser.WebContentsStatics;
import org.chromium.mojo.system.MojoException;
import org.chromium.services.service_manager.InterfaceFactory;
import org.chromium.url.Origin;
import org.jni_zero.CalledByNative;
import org.jni_zero.JNINamespace;
import org.jni_zero.NativeMethods;
import org.json.JSONArray;
import org.json.JSONException;
import org.json.JSONObject;

import java.nio.charset.StandardCharsets;
import java.security.MessageDigest;
import java.security.NoSuchAlgorithmException;

/**
 * A page's passkey request on Android (docs/passkeys.md, Our own passkeys): content hands
 * navigator.credentials.create and .get with publicKey to Java alone, so this is the frame's
 * blink.mojom.Authenticator. Android's own passkey providers don't answer us, and our own
 * passkeys are the only way: the request goes to Go as a dialog of kind passkey with steps, as
 * the Mac's does, with ours there and nothing else (own and only). The keys are Go's: this
 * side checks the site may ask for the RP ID, makes the client data, and carries Go's bytes to
 * the page. One request at a time for a frame.
 */
@JNINamespace("project_b")
final class Passkeys implements Authenticator {
    // The COSE algorithm of the keys Go makes: ES256.
    private static final int ES256 = -7;

    // The requests Go knows of, by its id. Main thread.
    private static final LongSparseArray<Passkeys> sRequests = new LongSparseArray<>();

    private final RenderFrameHost mFrame;

    // The request waiting: Go's id for it, 0 with none; which kind; its client data, and
    // what its step for ours carries; and the page's callback, one of the two.
    private long mId;
    private boolean mCreate;
    private byte[] mClientData;
    private JSONObject mOwn;
    private boolean mEchoCredProps;
    private MakeCredential_Response mMade;
    private GetCredential_Response mGot;

    private Passkeys(RenderFrameHost frame) {
        mFrame = frame;
    }

    /** Makes every frame's passkey requests ours. Once, before a page loads. */
    static void register() {
        InterfaceRegistrar.Registry.addRenderFrameHostRegistrar(
                (registry, frame) ->
                        registry.addInterface(
                                Authenticator.MANAGER,
                                (InterfaceFactory<Authenticator>) () -> new Passkeys(frame)));
    }

    private static String encode(byte[] bytes) {
        return Base64.encodeToString(bytes, Base64.URL_SAFE | Base64.NO_PADDING | Base64.NO_WRAP);
    }

    private static byte[] decode(JSONObject from, String key) throws JSONException {
        return Base64.decode(from.getString(key), Base64.URL_SAFE | Base64.NO_PADDING);
    }

    private static JSONArray ids(PublicKeyCredentialDescriptor[] credentials) {
        JSONArray out = new JSONArray();
        if (credentials != null) {
            for (PublicKeyCredentialDescriptor c : credentials) out.put(encode(c.id));
        }
        return out;
    }

    // An origin as a page's client data says it: no port when it is the scheme's own.
    private static String serialize(Origin origin) {
        int port = origin.getPort();
        boolean usual =
                port <= 0
                        || (port == 443 && "https".equals(origin.getScheme()))
                        || (port == 80 && "http".equals(origin.getScheme()));
        return origin.getScheme() + "://" + origin.getHost() + (usual ? "" : ":" + port);
    }

    // Whether the phone has a screen lock, which every use of a passkey is checked with.
    private static boolean hasScreenLock() {
        return ContextUtils.getApplicationContext()
                .getSystemService(KeyguardManager.class)
                .isDeviceSecure();
    }

    @Override
    public void makeCredential(
            PublicKeyCredentialCreationOptions options, MakeCredential_Response callback) {
        if (mId != 0) {
            callback.call(AuthenticatorStatus.PENDING_REQUEST, null, null);
            return;
        }
        boolean es256 = false;
        for (PublicKeyCredentialParameters p : options.publicKeyParameters) {
            es256 |= p.algorithmIdentifier == ES256;
        }
        // A key apart from the device isn't ours to be, and nothing else is here.
        boolean apart =
                options.authenticatorSelection != null
                        && options.authenticatorSelection.authenticatorAttachment
                                == AuthenticatorAttachment.CROSS_PLATFORM;
        if (!es256 || apart || options.isConditional) {
            callback.call(
                    es256
                            ? AuthenticatorStatus.NOT_ALLOWED_ERROR
                            : AuthenticatorStatus.ALGORITHM_UNSUPPORTED,
                    null,
                    null);
            return;
        }
        Origin origin = mFrame.getLastCommittedOrigin();
        String site = options.relyingParty.id;
        mFrame.performMakeCredentialWebAuthSecurityChecks(
                site,
                origin,
                options.isPaymentCredentialCreation,
                null,
                null,
                (checked) -> {
                    if (checked.securityCheckResult != AuthenticatorStatus.SUCCESS) {
                        callback.call(checked.securityCheckResult, null, null);
                        return;
                    }
                    JSONObject own = new JSONObject();
                    try {
                        JSONObject user = new JSONObject();
                        user.put("id", encode(options.user.id));
                        user.put("name", options.user.name == null ? "" : options.user.name);
                        user.put(
                                "displayName",
                                options.user.displayName == null ? "" : options.user.displayName);
                        own.put("user", user);
                        own.put("exclude", ids(options.excludeCredentials));
                    } catch (JSONException e) {
                        callback.call(AuthenticatorStatus.UNKNOWN_ERROR, null, null);
                        return;
                    }
                    mMade = callback;
                    mEchoCredProps = options.credProps;
                    start(
                            true,
                            site,
                            origin,
                            options.challenge,
                            checked.isCrossOrigin,
                            own,
                            new JSONArray());
                });
    }

    @Override
    public void getCredential(GetCredentialOptions options, GetCredential_Response callback) {
        PublicKeyCredentialRequestOptions asked = options.publicKey;
        if (mId != 0) {
            callback.call(failed(AuthenticatorStatus.PENDING_REQUEST));
            return;
        }
        // A sign-in field's list and a request that mustn't ask are not built: only a
        // request with a card.
        if (asked == null || options.mediation != Mediation.MODAL) {
            callback.call(failed(AuthenticatorStatus.NOT_ALLOWED_ERROR));
            return;
        }
        Origin origin = mFrame.getLastCommittedOrigin();
        mFrame.performGetAssertionWebAuthSecurityChecks(
                asked.relyingPartyId,
                origin,
                false,
                null,
                null,
                (checked) -> {
                    if (checked.securityCheckResult != AuthenticatorStatus.SUCCESS) {
                        callback.call(failed(checked.securityCheckResult));
                        return;
                    }
                    mGot = callback;
                    start(
                            false,
                            asked.relyingPartyId,
                            origin,
                            asked.challenge,
                            checked.isCrossOrigin,
                            new JSONObject(),
                            ids(asked.allowCredentials));
                });
    }

    // Tells Go of the request, with its key step: ours is there, and is the only way.
    private void start(
            boolean create,
            String site,
            Origin origin,
            byte[] challenge,
            boolean crossOrigin,
            JSONObject own,
            JSONArray allow) {
        WebContents contents = WebContentsStatics.fromRenderFrameHost(mFrame);
        String clientData =
                ClientDataJson.buildClientDataJson(
                        create
                                ? ClientDataRequestType.WEB_AUTHN_CREATE
                                : ClientDataRequestType.WEB_AUTHN_GET,
                        serialize(origin),
                        challenge,
                        crossOrigin,
                        null,
                        site,
                        contents == null ? null : contents.getMainFrame().getLastCommittedOrigin());
        long id =
                contents == null || clientData == null
                        ? 0
                        : PasskeysJni.get().open(contents, site, serialize(origin));
        if (id == 0) {
            end(AuthenticatorStatus.NOT_ALLOWED_ERROR);
            return;
        }
        mId = id;
        mCreate = create;
        mClientData = clientData.getBytes(StandardCharsets.UTF_8);
        mOwn = own;
        sRequests.put(id, this);
        try {
            JSONObject key = new JSONObject();
            key.put("request", create ? "create" : "get");
            key.put("site", site);
            key.put("phone", false);
            key.put("own", true);
            key.put("only", true);
            if (!create) {
                key.put("allow", allow);
                mOwn.put("clientDataHash", encode(sha256(mClientData)));
            }
            PasskeysJni.get().step(id, "key", key.toString());
        } catch (JSONException e) {
            end(AuthenticatorStatus.UNKNOWN_ERROR);
        }
    }

    private static byte[] sha256(byte[] bytes) {
        try {
            return MessageDigest.getInstance("SHA-256").digest(bytes);
        } catch (NoSuchAlgorithmException e) {
            throw new IllegalStateException(e);
        }
    }

    private static GetCredentialResponse failed(int status) {
        GetAssertionResponse r = new GetAssertionResponse();
        r.status = status;
        GetCredentialResponse out = new GetCredentialResponse();
        out.setGetAssertionResponse(r);
        return out;
    }

    // Ends the request, the page hearing status, and Go that the dialog closed.
    private void end(int status) {
        MakeCredential_Response made = mMade;
        GetCredential_Response got = mGot;
        long id = mId;
        mMade = null;
        mGot = null;
        mId = 0;
        mClientData = null;
        mOwn = null;
        if (id != 0) {
            sRequests.remove(id);
            PasskeysJni.get().closed(id);
        }
        if (made != null) made.call(status, null, null);
        if (got != null) got.call(failed(status));
    }

    /** Go's answer_passkey: what the person did at request id's step, or Go's answer to ours. */
    @CalledByNative
    static void answer(long id, String action, String text) {
        Passkeys request = sRequests.get(id);
        if (request != null) request.answered(action, text);
    }

    private void answered(String action, String text) {
        try {
            switch (action) {
                case "cancel":
                    end(AuthenticatorStatus.NOT_ALLOWED_ERROR);
                    break;
                case "own":
                    // The person chose: ours makes or signs, which is Go's to do.
                    PasskeysJni.get().step(mId, mCreate ? "ownMake" : "ownSign", mOwn.toString());
                    break;
                case "made":
                    if (mMade != null) made(new JSONObject(text));
                    break;
                case "signed":
                    if (mGot != null) signed(new JSONObject(text));
                    break;
                case "ownFailed":
                    end(
                            "excluded".equals(text)
                                    ? AuthenticatorStatus.CREDENTIAL_EXCLUDED
                                    : AuthenticatorStatus.NOT_ALLOWED_ERROR);
                    break;
                default:
                    break;
            }
        } catch (JSONException | IllegalArgumentException e) {
            end(AuthenticatorStatus.UNKNOWN_ERROR);
        }
    }

    private CommonCredentialInfo info(JSONObject from) throws JSONException {
        CommonCredentialInfo info = new CommonCredentialInfo();
        info.rawId = decode(from, "credentialId");
        info.id = encode(info.rawId);
        info.clientDataJson = mClientData;
        info.authenticatorData = decode(from, "authData");
        return info;
    }

    private void made(JSONObject from) throws JSONException {
        MakeCredentialAuthenticatorResponse r = new MakeCredentialAuthenticatorResponse();
        r.info = info(from);
        r.authenticatorAttachment = AuthenticatorAttachment.PLATFORM;
        r.attestationObject = decode(from, "attestationObject");
        r.transports = new int[] {AuthenticatorTransport.INTERNAL};
        r.publicKeyDer = decode(from, "publicKey");
        r.publicKeyAlgo = ES256;
        r.echoCredProps = mEchoCredProps;
        r.hasCredPropsRk = true;
        r.credPropsRk = true;
        MakeCredential_Response callback = mMade;
        mMade = null;
        end(AuthenticatorStatus.SUCCESS);
        callback.call(AuthenticatorStatus.SUCCESS, r, null);
    }

    private void signed(JSONObject from) throws JSONException {
        GetAssertionAuthenticatorResponse r = new GetAssertionAuthenticatorResponse();
        r.info = info(from);
        r.authenticatorAttachment = AuthenticatorAttachment.PLATFORM;
        r.signature = decode(from, "signature");
        r.userHandle = decode(from, "userHandle");
        r.extensions = new AuthenticationExtensionsClientOutputs();
        GetAssertionResponse response = new GetAssertionResponse();
        response.status = AuthenticatorStatus.SUCCESS;
        response.credential = r;
        GetCredentialResponse out = new GetCredentialResponse();
        out.setGetAssertionResponse(response);
        GetCredential_Response callback = mGot;
        mGot = null;
        end(AuthenticatorStatus.SUCCESS);
        callback.call(out);
    }

    @Override
    public void isUserVerifyingPlatformAuthenticatorAvailable(
            IsUserVerifyingPlatformAuthenticatorAvailable_Response callback) {
        callback.call(hasScreenLock());
    }

    @Override
    public void isConditionalMediationAvailable(IsConditionalMediationAvailable_Response callback) {
        callback.call(false);
    }

    @Override
    public void report(PublicKeyCredentialReportOptions options, Report_Response callback) {
        callback.call(AuthenticatorStatus.NOT_IMPLEMENTED, null);
    }

    @Override
    public void getClientCapabilities(GetClientCapabilities_Response callback) {
        boolean lock = hasScreenLock();
        String[] names = {
            "passkeyPlatformAuthenticator", "userVerifyingPlatformAuthenticator", "relatedOrigins",
            "hybridTransport", "conditionalGet", "conditionalCreate", "immediateGet"
        };
        boolean[] have = {lock, lock, true, false, false, false, false};
        WebAuthnClientCapability[] out = new WebAuthnClientCapability[names.length];
        for (int i = 0; i < names.length; i++) {
            out[i] = new WebAuthnClientCapability();
            out[i].name = names[i];
            out[i].supported = have[i];
        }
        callback.call(out);
    }

    /** The page took its request back. */
    @Override
    public void cancel() {
        if (mId != 0) end(AuthenticatorStatus.ABORT_ERROR);
    }

    /** The frame went: its page navigated, or its tab closed. */
    @Override
    public void close() {
        mMade = null;
        mGot = null;
        if (mId != 0) end(AuthenticatorStatus.NOT_ALLOWED_ERROR);
    }

    @Override
    public void onConnectionError(MojoException e) {
        close();
    }

    @NativeMethods
    interface Natives {
        // Tells Go of a request in contents' tab, as a dialog of kind passkey, and returns
        // Go's id for it, or 0 when the tab isn't one of Go's.
        long open(WebContents contents, String site, String origin);

        // A step of request id (PB_EVENT_PASSKEY_STEP).
        void step(long id, String name, String carries);

        // Request id ended (PB_EVENT_DIALOG_CLOSED).
        void closed(long id);
    }
}
