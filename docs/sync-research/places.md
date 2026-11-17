# Sync backends the user controls: verified facts

Researched 2026-11-17 for a Go-core browser on macOS and Android, where all data is encrypted end to end before it leaves the device. Every fact links to its source. **[unverified]** marks anything that rests on community posts, on my own inference, or that I could not confirm. Lines marked "Implication" are analysis, not sourced fact.

Because we encrypt everything ourselves, a backend only has to give us four things: somewhere to put bytes, a safe way for several devices to write, a cheap way to see what changed, and a sign-in the user can manage.

---

## 1. iCloud Drive and CloudKit

### Access and sign-in
- **macOS ubiquity container.** The app needs the iCloud entitlement and gets its container from `URLForUbiquityContainerIdentifier:`, called on a background thread ([Apple iCloud Design Guide](https://developer.apple.com/library/archive/documentation/General/Conceptual/iCloudDesignGuide/Chapters/DesigningForDocumentsIniCloud.html)). The capabilities iCloud documents, CloudKit, key-value storage and Push are available to both App Store and **Developer ID** Mac apps, but not to free developer accounts ([Apple supported capabilities, macOS](https://developer.apple.com/help/account/reference/supported-capabilities-macos), read from the page's table).
- **macOS, a folder the user picks** (for example `~/Library/Mobile Documents/com~apple~CloudDocs/...`). This is ordinary file I/O. With "Optimize Mac Storage" on, files "don't take up space on your Mac" and are downloaded only when needed ([Apple Support](https://support.apple.com/guide/mac-help/optimize-storage-space-sysp4ee93ca4/mac)), so a file we read may not be on disk yet.
- **CloudKit.** `CKSyncEngine` is available on macOS 14+ and iOS 17+. It needs the CloudKit and Remote-notifications entitlements, and Apple says not to use it for the public database ([CKSyncEngine docs](https://developer.apple.com/documentation/cloudkit/cksyncengine-5sie5), read from the docs JSON).
- **Android.** Apple has no Android SDK or public API for iCloud Drive; the only route is icloud.com in a browser ([anexplorer.io](https://pay.anexplorer.io/cloud/icloud), [Android Central](https://www.androidcentral.com/how-access-your-icloud-data-your-android-phone)) **[unverified: no Apple statement found]**.
- **CloudKit Web Services, the only non-Apple path.**
  - Requests carry a `ckAPIToken` plus a `ckWebAuthToken`. The web token comes from an Apple ID sign-in redirect flow and is rotated on every request: "Each token is intended for a single round trip". It lasts 30 minutes by default, or 2 weeks with "Keep me signed in". Server-to-server keys reach only the public database ([CloudKit Web Services Reference](https://developer.apple.com/library/archive/documentation/DataManagement/Conceptual/CloudKitWebServicesReference/SettingUpWebServices.html)).
  - With **Advanced Data Protection** on, iCloud.com web access is off by default ([Apple Support 102651](https://support.apple.com/en-us/102651)). One developer reported CloudKit JS getting `ACCESS_DENIED` "private db access disabled for this account" ([SO mirror](https://kiwix.lema.org/stackoverflow.com_en_all_2023-11/questions/75253111/cloudkit-js-issue-with-advanced-data-protection)) **[unverified, single report]**.
  - There are recent forum reports of 421 `AUTHENTICATION_REQUIRED` straight after a successful CloudKit JS sign-in ([Apple forums](https://developer.apple.com/forums/thread/841292)).
  - Implication: reaching CloudKit from Android is possible in principle but fragile, and it breaks for users who turn on ADP.

### Concurrency primitives
- **iCloud Drive files.** "The use of file coordinators and presenters is mandatory when working with iCloud documents." Conflicts arrive as `NSFileVersion` objects. On iOS the app must pick the winner and delete the losing versions. On macOS: "rely on the system to resolve document conflicts." Change discovery uses `NSMetadataQuery`. A Mac is a "greedy peer" that downloads automatically, while iOS does not ([iCloud Design Guide](https://developer.apple.com/library/archive/documentation/General/Conceptual/iCloudDesignGuide/Chapters/DesigningForDocumentsIniCloud.html)). There is no compare-and-swap.
- **CloudKit.**
  - A save fails with `serverRecordChanged` and returns the server's record for us to merge, which is per-record optimistic concurrency.
  - The server pushes a notification to other devices on every change, and `CKSyncEngine` listens for it and fetches.
  - Transient errors such as `requestRateLimited` are retried automatically ([WWDC23 session 10188](https://developer.apple.com/videos/play/wwdc2023/10188/)).
  - Each request holds at most **250 records** (saves plus deletes) ([CKSyncEngine docs](https://developer.apple.com/documentation/cloudkit/cksyncengine-5sie5)).
  - Sync timing is "indeterminate" because it depends on system conditions ([same](https://developer.apple.com/documentation/cloudkit/cksyncengine-5sie5)).

### Quotas and cost
- Private-database data "counts toward the user's iCloud storage quota" ([CKContainer.privateCloudDatabase](https://developer.apple.com/documentation/cloudkit/ckcontainer/privateclouddatabase)). It costs us nothing; the user pays through iCloud.
- Every user gets **5 GB** free, and iCloud+ plans offer 50 GB, 200 GB, 2 TB, 6 TB and 12 TB ([Apple Support 108047](https://support.apple.com/en-us/108047), [108922](https://support.apple.com/en-us/108922)).
- Public-database limits are not documented; Apple staff say "we don't have a documentation formally documenting the limits" ([search summary of Apple forums](https://developer.apple.com/forums/thread/740916)) **[unverified]**.

### Gotchas
- Files evicted under Optimize Mac Storage, mandatory file coordination, and conflict versions that use the user's quota until we delete them ([iCloud Design Guide](https://developer.apple.com/library/archive/documentation/General/Conceptual/iCloudDesignGuide/Chapters/DesigningForDocumentsIniCloud.html)).
- Under ADP, CloudKit "encrypted fields and assets are end-to-end encrypted" ([Apple Support 102651](https://support.apple.com/en-us/102651)). This is a bonus, but we encrypt anyway.
- Implication: iCloud is a good fit for Mac↔iPhone, but **not for Mac↔Android**.

---

## 2. Google Drive (REST v3)

### Access and sign-in
- **REST only, on both platforms.** The Drive Android API was deprecated in December 2018 and shut down on 1 February 2023; Google says to use the Drive REST API instead ([migration guide](https://developers.google.com/workspace/drive/api/guides/android-api-deprecation)).
- **Android sign-in.** Google recommends Credential Manager for sign-in and `AuthorizationClient` for scopes, requested at the moment they are needed ([Android authorization](https://developer.android.com/identity/authorization)).
- **Desktop sign-in.** Use the system browser with a loopback redirect. "Desktop apps are public clients … Always use PKCE" ([Google OAuth for native apps](https://developers.google.com/identity/protocols/oauth2/native-app)).
- **Scopes** ([Drive scope classes](https://developers.google.com/workspace/drive/api/guides/api-specific-auth)):
  - **Non-sensitive:** `drive.appdata`, `drive.file` (files the user opened or picked with the app) and `drive.install`.
  - **Restricted:** `drive`, `drive.readonly` and `drive.metadata`. These need "restricted scope OAuth App Verification", and if the data is stored or sent through servers, a **security assessment**.
  - That assessment is **annual**, done by "a Google empanelled group of security assessors" ([Google Cloud help](https://support.google.com/cloud/answer/13464321)).
  - Even apps with only non-sensitive scopes still need brand verification ([same](https://support.google.com/cloud/answer/13464321)).
- **Testing status.** An app in Testing is capped at **100 test users**, and their tokens expire after **7 days** ([Manage App Audience](https://support.google.com/cloud/answer/15549945)). There is also a limit of 100 refresh tokens per account per client ID ([OAuth 2.0 overview](https://developers.google.com/identity/protocols/oauth2)).
- **appDataFolder.** A hidden per-app folder: "only accessible by your app … hidden from the user and from other Google Drive apps". It can't be shared or trashed, and it "is deleted when a user uninstalls your app from their My Drive" ([appdata guide](https://developers.google.com/workspace/drive/api/guides/appdata)).
- **Android SAF.** Community reports say Drive's provider does not support `ACTION_OPEN_DOCUMENT_TREE`, only single files ([SO 2021 mirror](https://kiwix.lema.org/stackoverflow.com_en_all_2023-11/questions/65963597/google-drive-file-picker-wanted-but-get-a-local-file-picker)). CommonsWare says Drive "supports a subset of SAF functionality" ([CommonsWare 2019](https://commonsware.com/blog/2019/12/01/scoped-storage-stories-problems-saf.html)). **[unverified on current Android; test on a device]**

### Concurrency primitives
- **No ETag / If-Match in v3.** The File resource has no etag field ([files resource](https://developers.google.com/workspace/drive/api/reference/rest/v3/files)). Community posts confirm etag disappeared after v2 ([SO mirror](https://kiwix.lema.org/stackoverflow.com_en_all_2023-11/questions/41307155/google-drive-file-etag-in-v3)). **[I found no official statement that `If-Match` is rejected on v3; untested]**
- **What exists instead.**
  - `version` is "a monotonically increasing version number", plus `headRevisionId` and `md5Checksum` ([files resource](https://developers.google.com/workspace/drive/api/reference/rest/v3/files)). These let us detect a change after the fact, but not block a write.
  - **Create-if-absent via `files.generateIds`.** Pre-generate an ID; a repeated create with that ID returns **409 Conflict** ([create-file guide](https://developers.google.com/workspace/drive/api/guides/create-file), [bulk import guide](https://developers.google.com/workspace/cse/guides/bulk-import-files)). This is the one atomic primitive Drive offers.
  - Names are **not unique**: two files in one folder can share a name ([files resource](https://developers.google.com/workspace/drive/api/reference/rest/v3/files), [bulk import guide](https://developers.google.com/workspace/cse/guides/bulk-import-files)).
- **Change feed.** `changes.getStartPageToken`, then `changes.list` with `pageToken` / `newStartPageToken` ([manage changes](https://developers.google.com/workspace/drive/api/guides/manage-changes)). `spaces` accepts `drive` and `appDataFolder` ([changes.list](https://developers.google.com/workspace/drive/api/reference/rest/v3/changes/list)).
- **Push.** `changes.watch` needs an **HTTPS webhook with a valid certificate**. Channels last at most 1 week for changes and 1 day for files ([push guide](https://developers.google.com/workspace/drive/api/guides/push)). Implication: push needs a server, so without our own we poll.

### Quotas and cost
- **API quota** ([limits](https://developers.google.com/workspace/drive/api/guides/limits)):
  - 1,000,000 quota units per minute per project, and 325,000 per minute per user. The page's own MCP section lists these the other way round.
  - Cost per call: read 5 units, list 100, write 50, download 200.
  - A 400,000,000 units per day threshold, and 1 TB egress per day.
  - Uploads are capped at 750 GB per user per day.
  - On 403/429, back off exponentially.
- **Storage.** 15 GB free, shared with Gmail and Photos ([Google One help](https://support.google.com/googleone/answer/9312312)).

### Gotchas
- No compare-and-swap on overwrite, duplicate names, push only through a webhook, and the hidden folder is lost if the user removes the app.
- Implication: design so that each device writes only its own files (or only new, immutable files with pre-generated IDs). Then no two devices ever overwrite the same file.

---

## 3. Dropbox (API v2)

### Access and sign-in
- **OAuth.** Use the code flow with **PKCE** for desktop and mobile apps, which can't keep a secret. Access tokens are short-lived; request `token_access_type=offline` to get a refresh token ([Dropbox OAuth guide](https://docs.dropboxapi.com/dropbox-api/docs/oauth)).
- **App Folder permission.** The app is limited to "data within its app folder only" ([same](https://docs.dropboxapi.com/dropbox-api/docs/oauth)). That folder is "a dedicated folder named after your app … within the Apps folder" ([developer guide](https://docs.dropboxapi.com/dropbox-api/docs/developer-resources/developer-guide)).
- **Development status and review.** A new app can link up to **500 users**. Once it reaches **50 users**, there are 2 weeks to apply for production, or linking freezes ([developer guide](https://docs.dropboxapi.com/dropbox-api/docs/developer-resources/developer-guide)).
- **Android SAF.** The Dropbox app does not support `ACTION_OPEN_DOCUMENT_TREE` ([Dropbox forum](https://www.dropboxforum.com/discussions/101000042/why-using-intent-action-open-document-does-not-list-the-dropbox/209654)) **[community, dated]**. Use HTTP from the Go core.

### Concurrency primitives
These are all from the API spec, [files.stone](https://raw.githubusercontent.com/dropbox/dropbox-api-spec/main/files.stone):
- **`WriteMode.update(rev)`** means "Overwrite if the given "rev" matches the existing file's "rev"". This is real compare-and-swap.
  - `strict_conflict` makes the check strict: it always conflicts on a rev mismatch, even if the file was deleted or the content is identical.
  - `autorename` makes the server rename instead of fail.
  - `WriteMode.add` means "Do not overwrite an existing file if there is a conflict", which is create-if-absent.
- **`content_hash`** checks the upload's integrity.
- **Change feed.** Use a cursor with `list_folder/continue`. `list_folder/longpoll` blocks until changes arrive; its timeout is 30 to 480 s (default 30) plus up to 90 s of jitter, and it may return a `backoff` value to honour.
- **Uploads.** Each request carries at most 150 MiB. An upload session holds up to about 2 TiB and lasts 7 days.
- **Write contention.** `429 too_many_write_operations` comes from lock contention on the namespace and has `Retry-After: 0`. A true `too_many_requests` 429 means honour `Retry-After` ([DBX performance guide](https://developers.dropbox.com/dbx-performance-guide), [Dropbox forum](https://www.dropboxforum.com/t5/Dropbox-API-Support-Feedback/dropbox-create-folder-v2-api-keeps-getting-429-too-many-write/m-p/707766)). Dropbox publishes no request numbers **[unverified that none exist]**.

### Quotas and cost
- Basic is free with **2 GB** and up to **3 devices**; Plus is US$9.99/month for 2 TB ([dropbox.com/basic](https://www.dropbox.com/en_GB/basic), [individual plans](https://www.dropbox.com/individual)).
- Dropbox staff say third-party API apps **don't count** toward the 3-device limit ([Dropbox forum](https://www.dropboxforum.com/t5/Dropbox-API-Support-Feedback/Three-devices-limit-and-API-access/m-p/336570)) **[staff forum post, 2019]**.

### Gotchas
- Production review at 50 users, and the small free tier.
- Implication: the best primitive set of the consumer clouds, because it has compare-and-swap, create-if-absent and a long-poll change feed with no server of ours.

---

## 4. OneDrive (Microsoft Graph), briefly

- **App Folder.** Requesting `Files.ReadWrite.AppFolder` gives the app a folder under `Apps/`, created on the first call to `/drive/special/approot`. The user can see it and change it, "like any other folder" ([App Folder docs](https://learn.microsoft.com/en-us/onedrive/developer/rest-api/concepts/special-folders-appfolder)).
- **Conditional writes.** On `PATCH` of an item, an `if-match` eTag or cTag that doesn't match returns **412** ([driveItem update](https://learn.microsoft.com/en-us/graph/api/driveitem-update)). Simple upload takes files up to 250 MB, and its page lists no `if-match` header ([put content](https://learn.microsoft.com/en-us/graph/api/driveitem-put-content)) **[whether content upload honours If-Match is unverified]**.
- **Delta.** Call `delta` and follow `@odata.nextLink`; the last page gives `@odata.deltaLink`. `token=latest` skips enumerating what already exists. A stale token returns **410 Gone** with a resync code. Always track items by id ([driveItem delta](https://learn.microsoft.com/en-us/graph/api/driveitem-delta)). A delta path under approot is listed ([App Folder docs](https://learn.microsoft.com/en-us/onedrive/developer/rest-api/concepts/special-folders-appfolder)).
- **Cost.** **5 GB** free, shared with Outlook attachments ([Microsoft storage FAQ](https://support.microsoft.com/en-us/onedrive/microsoft-storage-faqs)).

---

## 5. WebDAV and Nextcloud

### Access and sign-in
- Plain HTTPS from the Go core on both platforms.
- **Nextcloud Login Flow v2.** POST `/index.php/login/v2`, open the login URL in the browser, then poll; the result is an **app password**. The token is valid for **20 minutes** ([Login Flow](https://docs.nextcloud.com/server/latest/developer_manual/client_apis/LoginFlow/index.html)).
- Basic auth with an app password is needed where SSO or 2FA is on ([WebDAV basics](https://docs.nextcloud.com/server/latest/developer_manual/client_apis/WebDAV/basic.html)).
- **Android SAF.** One user says the Nextcloud Android app is a full document provider ([Logseq forum](https://discuss.logseq.com/t/use-storage-access-framework-for-best-privacy-compatibility/6462)) **[unverified for tree write]**.

### Concurrency primitives
- **Properties.** Nextcloud exposes `getetag`, `oc:fileid` and `oc:size`, and accepts `X-OC-MTime` ([WebDAV basics](https://docs.nextcloud.com/server/latest/developer_manual/client_apis/WebDAV/basic.html)). The page does not document `If-Match`.
- **If-Match in practice.** A Nextcloud client log shows "An If-Match header was specified, but none of the specified ETags matched" ([Nextcloud forum](https://help.nextcloud.com/t/issues-with-syncing-files-larger-than-10mb-on-linux/233815)), and SabreDAV's client guide uses `If-Match` on PUT and DELETE ([sabre.io](https://sabre.io/dav/building-a-caldav-client/)). **[treat as likely-supported; test against the user's server]**
- **Create-if-absent rename.** WebDAV `MOVE` with `Overwrite: F` "MUST fail with a 412" if the destination exists, and all DAV-compliant resources must support it ([RFC 4918 §10.6](https://www.rfc-editor.org/rfc/rfc4918#section-10.6)).
- **Change feed.** WebDAV-Sync (`sync-collection`) can be used only "if the server reports that it has support for it", and an old token may be forgotten, so be ready to resync fully ([sabre.io](https://sabre.io/dav/building-a-caldav-client/)). Whether Nextcloud offers it for files is **[unverified]**; Nextcloud folder ETags are commonly used instead **[unverified]**.
- **Push.** Nextcloud `notify_push` needs **Redis** and a reverse proxy, and is "best effort", so clients should still poll ([notify_push](https://github.com/nextcloud/notify_push)).

### Nextcloud apps
- **E2EE app.** E2EE files "are not accessible with the web interface and other WebDAV clients", and the app is not compatible with server-side encryption ([end_to_end_encryption](https://github.com/nextcloud/end_to_end_encryption)). Implication: we must not depend on it; we bring our own encryption.
- **Bookmarks app.**
  - A REST API for "third-party clients" ([docs](https://nextcloud-bookmarks.readthedocs.io/en/latest/)).
  - A lock at `POST/DELETE /index.php/apps/bookmarks/public/rest/v2/lock`, released automatically after **30 min** ([locking](https://nextcloud-bookmarks.readthedocs.io/en/latest/locking.html)).
  - It stores bookmarks in plaintext on the server **[inference]**.
- **Passwords app.** It has a documented API and optional client-side encryption with a master password ([GitHub README](https://github.com/marius-wieschollek/passwords), [Nextcloud forum](https://help.nextcloud.com/t/encryption-passwords-app/60492?page=2)) **[API details not verified]**.

---

## 6. A git repository

- **Real git.** "git push refuses to update a remote ref that is not an ancestor of the local ref" ([git-push](https://git-scm.com/docs/git-push)). That makes a non-fast-forward push an atomic compare-and-swap on the branch, and `--force-with-lease=<ref>:<expect>` makes the CAS explicit.
- **GitHub REST.**
  - Updating a file needs `sha` ("The blob SHA of the file being replaced").
  - GitHub warns that concurrent create, update or delete calls "will conflict", so call them one after another.
  - GET returns full content up to 1 MB, raw only from 1 to 100 MB, and nothing above 100 MB.
  - A directory listing stops at **1,000** files ([contents API](https://docs.github.com/en/rest/repos/contents)).
  - Updating a ref with `force=false` "make[s] sure you're not overwriting work" ([git refs API](https://docs.github.com/en/rest/git/refs)).
- **Rate limits** ([REST rate limits](https://docs.github.com/en/rest/using-the-rest-api/rate-limits-for-the-rest-api)):
  - **5,000/h** for a user token, and 60/h without one.
  - Secondary limits: 100 concurrent requests, 900 points/min per endpoint (writes cost 5), and about **80 content-creating requests/min and 500/h**.
- **Auth.**
  - A **GitHub App device flow** must be enabled in the app's settings and needs no client secret. User tokens last **8 h** and refresh tokens **6 months**. Access is the intersection of what the app and the user can reach ([GitHub App user tokens](https://docs.github.com/en/apps/creating-github-apps/authenticating-with-a-github-app/generating-a-user-access-token-for-a-github-app)).
  - **Fine-grained PATs** can be limited to selected repositories with `contents: write`; the default expiry is 30 days, and a user can hold up to 50 ([PAT docs](https://docs.github.com/en/authentication/keeping-your-account-and-data-secure/managing-your-personal-access-tokens)).
  - GitHub Apps need no Google-style review **[inference]**.
- **Size.** GitHub warns above 50 MiB per file and blocks above 100 MiB. Repos should ideally stay **< 1 GB**, and staying under 5 GB is strongly recommended ([large files](https://docs.github.com/en/repositories/working-with-files/managing-large-files/about-large-files-on-github)). GitLab.com Free gives **10 GiB** per project, after which the project goes read-only ([GitLab storage](https://docs.gitlab.com/user/storage_usage_quotas/)).
- **Gotchas.** History grows forever; with encrypted blobs, git's delta compression gains nothing **[inference]**. There is no push notification without webhooks to a server.

---

## 7. S3-style buckets

- **AWS S3 conditional writes** ([conditional writes](https://docs.aws.amazon.com/AmazonS3/latest/userguide/conditional-writes.html)):
  - **`If-None-Match: *`** arrived on 20 Aug 2024 ([What's New](https://aws.amazon.com/about-aws/whats-new/2024/08/amazon-s3-conditional-writes)).
  - **`If-Match: <etag>`** arrived on 25 Nov 2024, along with bucket-policy enforcement through `s3:if-match` / `s3:if-none-match` ([What's New](https://aws.amazon.com/about-aws/whats-new/2024/11/amazon-s3-functionality-conditional-writes), [enforcement](https://aws.amazon.com/about-aws/whats-new/2024/11/amazon-s3-enforcement-conditional-write-operations-general-purpose-buckets)). Copy operations followed in Oct 2025 ([What's New](https://aws.amazon.com/about-aws/whats-new/2025/10/amazon-s3-conditional-write-functionality-copy-operations)).
  - They work on PutObject, CompleteMultipartUpload and CopyObject.
  - A mismatch returns **412**. When writers race, "the first write operation to finish succeeds". A racing delete can return **409** (retry the PutObject) or 404.
  - Requests must use SigV4. `If-Match` also needs `s3:GetObject`.
- **Consistency.** S3 has had strong read-after-write consistency since Dec 2020 ([What's New](https://aws.amazon.com/about-aws/whats-new/2020/12/amazon-s3-now-delivers-strong-read-after-write-consistency-automatically-for-all-applications/)).
- **Presigned URLs.** They last up to **7 days** with IAM-user SigV4 credentials, no longer than temporary credentials, and are bearer tokens ([presigned URLs](https://docs.aws.amazon.com/AmazonS3/latest/userguide/using-presigned-url.html)).
- **AWS cost.**
  - GET costs $0.0004 per 1,000, and the first 100 GB/month of egress is free ([S3 pricing](https://aws.amazon.com/s3/pricing/)).
  - Standard storage is $0.023/GB-month and PUT/LIST $0.005 per 1,000 in us-east-1 ([third-party summaries, e.g. upstash](https://upstash.com/blog/aws-s3-pricing-explained-2026)) **[not read off the AWS page]**.
  - New customers get up to $200 in free-tier credits ([S3 pricing](https://aws.amazon.com/s3/pricing/)).
- **Cloudflare R2.**
  - PutObject supports `If-Match`, `If-None-Match`, `If-Modified-Since` and `If-Unmodified-Since`, "evaluated atomically against the committed bucket state" ([R2 S3 API](https://developers.cloudflare.com/r2/api/s3/api/)).
  - S3 bucket-notification APIs are not implemented ([same](https://developers.cloudflare.com/r2/api/s3/api/)). R2 instead has event notifications into a Queue, which can be pulled over HTTP ([R2 event notifications](https://developers.cloudflare.com/r2/buckets/event-notifications/)). The plan requirements for that are **[unverified]**.
  - Pricing: $0.015/GB-month, Class A $4.50/M and Class B $0.36/M. The monthly free tier is 10 GB, 1M Class A and 10M Class B, and **egress is free** ([R2 pricing](https://developers.cloudflare.com/r2/pricing/)).
- **Backblaze B2.**
  - Storage costs $6.95/TB-month, with the first 10 GB free. Class A, B and C calls are free; Class D costs $0.004 per 10k. Egress is free up to 3× stored data ([B2 pricing](https://www.backblaze.com/cloud-storage/pricing)).
  - B2 **does not support `If-None-Match`** on PutObject, per Proxmox's 2025 patches ([pbs-devel](https://lists.proxmox.com/pipermail/pbs-devel/2025-July/014234.html)) **[indirect; check B2 docs or test]**.
- **MinIO.**
  - Its docs list `If-Match` and `If-None-Match` on PutObject ([MinIO S3 compatibility](https://docs.min.io/aistor/developers/s3-api-compatibility/)).
  - **The `minio/minio` GitHub repo is archived** (checked through the GitHub API on 2026-11-17: `archived: true`). Community binaries stopped in 2025 ([InfoQ](https://infoq.com/news/2025/12/minio-s3-api-alternatives/)).
  - Implication: don't recommend MinIO for self-hosting.
- **Access.** HTTPS with SigV4 from Go on both platforms. The user pastes an access key and secret; there is no OAuth. Polling uses ListObjectsV2.

---

## 8. Device to device on the local network

- **macOS 15 Local Network privacy** ([TN3179](https://developer.apple.com/documentation/technotes/tn3179-understanding-local-network-privacy), read from the docs JSON):
  - Introduced in macOS 15. The first local operation shows an alert, and the operation "may [be denied] immediately, before the user has responded", so retry or wait for connectivity.
  - Bonjour register, browse and resolve all need permission. So do outgoing TCP and all UDP sends; **listening for incoming TCP does not**.
  - Required keys: `NSLocalNetworkUsageDescription`, and `NSBonjourServices` for the Bonjour types.
  - Exempt: launchd daemons, root, and CLI tools run from Terminal. LaunchAgents are not exempt.
  - The OS tracks the app by code signature, so it must be properly signed.
  - macOS 15.1 fixed many bugs.
- **Android local network protection** ([Android docs](https://developer.android.com/privacy-and-security/local-network-permission)):
  - It is **enforced for apps targeting Android 17 (API 37)**, through the runtime permission **`ACCESS_LOCAL_NETWORK`** in the `NEARBY_DEVICES` group.
  - It covers outgoing and incoming TCP, UDP unicast, multicast and broadcast, and `.local` resolution, across all networking stacks.
  - Exemption: `NsdManager` `DiscoveryRequest.FLAG_SHOW_PICKER` lets the user pick a single device, and the app can then connect to it without the permission.
  - When blocked, TCP times out and UDP fails with `EPERM`.
  - Apps targeting ≤36 keep implicit access. The Android 16 opt-in (`NEARBY_WIFI_DEVICES`) is deprecated.
- **`NEARBY_WIFI_DEVICES` (Android 13+)** covers Wi-Fi Aware, P2P and local-only hotspot APIs, and can be declared `neverForLocation` ([Wi-Fi permissions](https://developer.android.com/develop/connectivity/wifi/wifi-permissions)).
- **Android background.** The shortest `PeriodicWorkRequest` interval is **15 minutes**, and when it runs depends on constraints and system optimisation ([WorkManager](https://developer.android.com/develop/background-work/background-tasks/persistent/getting-started/define-work)). Implication: phone-to-Mac LAN sync happens while the app is open, not on a timer.
- **Prior art: Syncthing BEP** ([BEP v1](https://docs.syncthing.net/specs/bep-v1.html), [relaying](https://docs.syncthing.net/users/relaying.html), [syncing](https://docs.syncthing.net/users/syncing.html)):
  - Mutual TLS 1.3; the device ID is the SHA-256 of the device certificate.
  - Each file carries a version vector. Blocks are 128 KiB to 16 MiB.
  - Relays are used only when a direct connection fails. They stay "end to end encrypted", but the relay sees IP, device ID and traffic volume.
  - On a conflict, the older-mtime copy is renamed `.sync-conflict-<date>-<time>-<modifiedBy>`.
  - The official Android app was discontinued in Dec 2024 because of Google Play publishing problems; Syncthing-Fork continues ([syncthing-android](https://github.com/syncthing/syncthing-android), [AlternativeTo](https://alternativeto.net/news/2024/10/the-syncthing-android-app-is-being-discontinued-but-alternatives-remain-available)).
- **QR pairing.** Implication: a QR code carrying the certificate fingerprint plus a one-time secret is the same model as Syncthing's device IDs **[design note, not sourced]**.

---

## 9. Self-hosted server

- **Firefox syncstorage-rs** ([Docker how-to](https://mozilla-services.github.io/syncstorage-rs/how-to/how-to-run-with-docker.html)):
  - Published images use MySQL or PostgreSQL, are `linux/amd64` only, and need a `SYNC_MASTER_SECRET`.
  - **Auth still goes through Mozilla accounts** (`oauth.accounts.firefox.com`).
  - Implication: reusing it ties our users to Mozilla's identity service, so it isn't really user-controlled.
- **Its protocol is still good prior art** ([Sync Storage API 1.5](https://mozilla-services.readthedocs.io/en/latest/storage/apis-1.5.html)):
  - `X-If-Unmodified-Since` → 412.
  - `GET /info/collections` returns last-modified per collection.
  - Atomic multi-POST batches with `batch=true` and then `commit=true`.
  - Payloads of at least 256 KiB must be accepted.
- **Our own Go binary.** We could copy those semantics. It gives the strongest primitives (compare-and-swap plus a long-poll or WebSocket change feed) but needs the user to run it. Taildrop or tsnet (below) could make it reachable without port-forwarding **[design note]**.

---

## 10. Short notes on other options

- **Passkey storage.**
  - largeBlob reportedly works in Google Password Manager and iCloud Keychain, but 1Password and Enpass decline it ([Corbado](https://www.corbado.com/blog/webauthn-l3-credential-manager-support)).
  - PRF is on by default in GPM and works with iCloud Keychain on macOS 15+ ([Corbado PRF](https://www.corbado.com/blog/passkeys-prf-webauthn)).
  - **[vendor blogs; no official size limits found]** Implication: use PRF to *derive the sync key* on each device. Don't use it to carry data.
- **USB stick or file export.** SAF works for a user-picked file. Simple, offline, manual **[no source needed]**.
- **Tailscale.**
  - Taildrop is in **alpha**, works only between one user's own devices, and has CLI `tailscale file cp/get`; resume does not work when the receiver is macOS or iOS ([Taildrop](https://tailscale.com/kb/1106/taildrop)).
  - **`tsnet`** embeds Tailscale in a Go program through a userspace network stack, with interactive or auth-key login ([tsnet](https://tailscale.com/kb/1244/tsnet)).
  - The Personal plan is free for up to 6 users ([pricing](https://tailscale.com/pricing)).
- **Email to self.** Gmail's API scopes `gmail.insert`, `gmail.modify` and `gmail.readonly` are all **restricted**, which means the annual assessment ([Gmail scopes](https://developers.google.com/workspace/gmail/api/auth/scopes)). Generic IMAP with an app password avoids that **[not researched further]**.
- **IPFS.** Content survives only while some node **pins** it. Garbage collection removes unpinned data, and pinning services may shut down ([IPFS persistence](https://docs.ipfs.tech/concepts/persistence/)). Implication: not a store the user controls unless they run a node.

---

## Summary table (facts from above)

| Backend | Android path | Compare-and-swap write | Change feed / push without our server | Sign-in burden |
|---|---|---|---|---|
| iCloud Drive | none | no (file coordination, NSFileVersion) | NSMetadataQuery (Apple only) | entitlement only |
| CloudKit | Web Services, fragile, blocked by ADP default | yes, per record (`serverRecordChanged`) | yes, push + CKSyncEngine (Apple only) | entitlement only |
| Google Drive | REST | **no** (create-if-absent only, via generateIds) | changes.list poll; push needs a webhook | OAuth + brand verification; appdata/file are non-sensitive |
| Dropbox | REST | **yes** (`update(rev)`, `add`) | cursor + **longpoll** | OAuth PKCE; production review at 50 users |
| OneDrive | REST | yes on metadata PATCH; content **[unverified]** | delta | Entra app registration |
| WebDAV/Nextcloud | HTTP | likely If-Match **[test]**; MOVE Overwrite:F | sync-collection **[unverified for files]**; notify_push | app password / Login Flow v2 |
| git | HTTP or real git | yes (fast-forward-only ref update) | none (poll) | device flow or PAT |
| S3 / R2 | HTTP SigV4 | **yes** (If-Match / If-None-Match); B2 no | list poll; R2 queue events | access keys |
| LAN P2P | sockets | ours to design | ours | pairing; OS local-network prompts |
| Self-hosted Go | HTTP | ours | ours | ours |
