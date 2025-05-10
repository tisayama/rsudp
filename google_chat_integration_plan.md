# RSUDP Notification Services Integration Plan

This document outlines the plan for integrating Google Chat, Discord, Amazon SNS, and LINE notifications into RSUDP.

## Objective

Create new consumer threads that send notifications about seismic events detected by RSUDP to specified Google Chat spaces, Discord channels, Amazon SNS topics, and LINE users/groups. Image notifications will be supported for Discord (direct upload), and for Google Chat & LINE (via S3 upload).

## Design: Google Chat

*   **Class Name:** `GoogleChatter` (inherits from `rsudp.raspberryshake.ConsumerThread`)
*   **File Name:** `rsudp/c_googlechat.py`
*   **Dependencies:** `requests`, `boto3` (for S3 image upload)
*   **Configuration:**
    *   `googlechat_webhook_url`: (string) Webhook URL.
    *   `extra_text`: (string, optional)
    *   `send_images`: (boolean, optional, default: `false`) Whether to attempt S3 upload for images.
    *   `s3_bucket_name`: (string, optional) S3 bucket for images (required if `send_images` is true).
    *   `s3_object_key_prefix`: (string, optional) Prefix for S3 object keys.
    *   `s3_aws_region`: (string, optional) S3 bucket region.
    *   `s3_upload_timeout_seconds`: (integer, optional, default: 3) Timeout for S3 upload.
    *   `aws_access_key_id`: (string, optional) For S3.
    *   `aws_secret_access_key`: (string, optional) For S3.

### Key Methods and Functionality (Google Chat)

*   **`__init__`**: Initializes Webhook, S3 client (if `send_images` and `s3_bucket_name` are set), queue, etc.
*   **`getq(self)`**: Standard.
*   **`_upload_image_to_s3(self, local_file_path, object_key)`**: (New private method) Uploads image to S3, returns public URL or None. Handles timeout.
*   **`_when_alarm(self, d)`**: Formats and sends a simple text message (JST, Japanese).
*   **`_when_img(self, d)`**:
    *   If `send_images` is true and S3 configured:
        *   Calls `_upload_image_to_s3()`.
        *   If upload successful, sends a Google Chat message including the S3 image URL.
        *   If upload fails or times out, sends nothing (alarm text is handled by `_when_alarm`).
    *   Else, ignores `IMGPATH`.
*   **`run(self)`**: Standard.

### Workflow Diagram (Google Chat - Mermaid)

```mermaid
graph TD
    subgraph RSUDP Core
        Producer[p_producer] -- Data --> Consumer[c_consumer]
    end

    subgraph GoogleChatter Module (c_googlechat.py)
        Consumer -- Queue --> GC(GoogleChatter)
        GC -- Reads Queue --> MsgType{Message Type?}
        MsgType -- ALARM --> AlarmG[_when_alarm]
        MsgType -- IMGPATH --> ImgG[_when_img]
        MsgType -- TERM --> TerminateG[Exit Thread]

        AlarmG -- Formats Text --> PrepareJsonTextG[Prepare JSON (Text)]
        PrepareJsonTextG -- requests.post --> GoogleAPI{Google Chat API (Webhook)}

        ImgG -- send_images? --> UploadS3G[_upload_image_to_s3]
        UploadS3G -- Success (URL) --> PrepareImgMsgG[Prepare JSON (Text with Image URL)]
        PrepareImgMsgG -- requests.post --> GoogleAPI
        UploadS3G -- Fail/Timeout --> LogWarnG[Log Warning, Send Nothing]
        
        GoogleAPI -- Response --> HandleRespG{Handle Response/Error}
        HandleRespG -- Retry? --> GoogleAPI
    end

    style GoogleAPI fill:#f9f,stroke:#333,stroke-width:2px
    style UploadS3G fill:#FF9900,stroke:#232F3E,stroke-width:1px
```

## Design: Discord

*   **Class Name:** `Discorder` (inherits from `rsudp.raspberryshake.ConsumerThread`)
*   **File Name:** `rsudp/c_discord.py`
*   **Dependencies:** `requests` library
*   **Configuration:** `discord_webhook_url`, `discord_use_embed`, `discord_send_images` (direct upload), `discord_extra_text`.

### Key Methods and Functionality (Discord)

*   **`__init__`**: Standard.
*   **`getq(self)`**: Standard.
*   **`_send_message(self, payload=None, files=None)`**: Standard.
*   **`_when_alarm(self, d)`**: Formats and sends Embed or text message (JST, Japanese).
*   **`_when_img(self, d)`**: If `send_images` is true, prepares local file and sends it directly with `requests.post` (no S3 involved for Discord).
*   **`run(self)`**: Standard.

### Workflow Diagram (Discord - Mermaid)

```mermaid
graph TD
    subgraph RSUDP Core
        Producer[p_producer] -- Data --> Consumer[c_consumer]
    end

    subgraph Discorder Module (c_discord.py)
        Consumer -- Queue --> DC(Discorder)
        DC -- Reads Queue --> MsgType{Message Type?}
        MsgType -- ALARM --> AlarmD[_when_alarm]
        MsgType -- IMGPATH --> ImgD[_when_img]
        MsgType -- TERM --> TerminateD[Exit Thread]

        AlarmD -- Formats Embed/Text --> PrepareMsgD[Prepare JSON (Embed/Text)]
        PrepareMsgD -- _send_message --> SendHelperD[_send_message]

        ImgD -- send_images? --> PrepareFileD[Prepare Local File for Upload]
        PrepareFileD -- _send_message --> SendHelperD

        SendHelperD -- requests.post --> DiscordAPI{Discord API (Webhook)}
        DiscordAPI -- Response --> HandleRespD{Handle Response/Error}
        HandleRespD -- Retry? --> SendHelperD
    end

    style DiscordAPI fill:#7289DA,stroke:#FFF,stroke-width:2px
```

## Design: Amazon SNS

*   **Class Name:** `SNSNotifier` (inherits from `rsudp.raspberryshake.ConsumerThread`)
*   **File Name:** `rsudp/c_sns.py`
*   **Dependencies:** `boto3` library
*   **Configuration:** `enabled`, `topic_arn`, `aws_access_key_id` (optional), `aws_secret_access_key` (optional), `aws_region`, `extra_text`.

### Key Methods and Functionality (Amazon SNS)

*   **`__init__`**: Standard.
*   **`getq(self)`**: Standard.
*   **`_send_sns_message(self, message_body)`**: Standard.
*   **`_when_alarm(self, d)`**: Formats and sends a simple text message (JST, Japanese, SMS-optimized).
*   **`_when_img(self, d)`**: Ignores `IMGPATH`.
*   **`run(self)`**: Standard.

### Workflow Diagram (Amazon SNS - Mermaid)

```mermaid
graph TD
    subgraph RSUDP Core
        Producer[p_producer] -- Data --> Consumer[c_consumer]
    end

    subgraph SNSNotifier Module (c_sns.py)
        Consumer -- Queue --> SN(SNSNotifier)
        SN -- Reads Queue --> MsgType{Message Type?}
        MsgType -- ALARM --> AlarmS[_when_alarm]
        MsgType -- IMGPATH --> IgnoreImgS[Ignore IMGPATH]
        MsgType -- TERM --> TerminateS[Exit Thread]

        AlarmS -- Formats Message --> PrepareSnsMsg[Prepare SNS Message Body]
        PrepareSnsMsg -- _send_sns_message --> SendHelperS[_send_sns_message]

        SendHelperS -- boto3.publish() --> SnsAPI{AWS SNS API}
        SnsAPI -- Response --> HandleRespS{Handle Response/Error}
        HandleRespS -- Retry? --> SendHelperS
    end

    style SnsAPI fill:#FF9900,stroke:#232F3E,stroke-width:2px
```

## Design: LINE

*   **Class Name:** `LINENotifier` (inherits from `rsudp.raspberryshake.ConsumerThread`)
*   **File Name:** `rsudp/c_line.py`
*   **Dependencies:** `line-bot-sdk-python`, `boto3` (for S3 image upload)
*   **Configuration:**
    *   `enabled`: (boolean)
    *   `channel_access_token`: (string)
    *   `to_ids`: (string) Comma-separated IDs.
    *   `extra_text`: (string, optional)
    *   `send_images`: (boolean, optional, default: `true`) Whether to attempt S3 upload for images.
    *   `s3_bucket_name`: (string, optional) S3 bucket for images (required if `send_images` is true).
    *   `s3_object_key_prefix`: (string, optional)
    *   `s3_aws_region`: (string, optional)
    *   `s3_upload_timeout_seconds`: (integer, optional, default: 3)
    *   `aws_access_key_id`: (string, optional) For S3.
    *   `aws_secret_access_key`: (string, optional) For S3.

### Key Methods and Functionality (LINE)

*   **`__init__`**: Initializes LINE Bot API client, S3 client (if `send_images` and `s3_bucket_name` are set), queue, etc.
*   **`getq(self)`**: Standard.
*   **`_upload_image_to_s3(self, local_file_path, object_key)`**: (New private method) Uploads image to S3, returns public URL or None. Handles timeout. (Similar to GoogleChatter's method, but implemented independently).
*   **`_send_line_message(self, message_objects)`**: Sends one or more LINE message objects (e.g., `TextMessage`, `ImageSendMessage`).
*   **`_when_alarm(self, d)`**: Formats and sends a `TextMessage` (JST, Japanese).
*   **`_when_img(self, d)`**:
    *   If `send_images` is true and S3 configured:
        *   Calls `_upload_image_to_s3()`.
        *   If upload successful, creates an `ImageSendMessage` with the S3 URL and sends it.
        *   If upload fails or times out, sends nothing (alarm text is handled by `_when_alarm`).
    *   Else, ignores `IMGPATH`.
*   **`run(self)`**: Standard.

### Workflow Diagram (LINE - Mermaid)

```mermaid
graph TD
    subgraph RSUDP Core
        Producer[p_producer] -- Data --> Consumer[c_consumer]
    end

    subgraph LINENotifier Module (c_line.py)
        Consumer -- Queue --> LN(LINENotifier)
        LN -- Reads Queue --> MsgType{Message Type?}
        MsgType -- ALARM --> AlarmL[_when_alarm]
        MsgType -- IMGPATH --> ImgL[_when_img]
        MsgType -- TERM --> TerminateL[Exit Thread]

        AlarmL -- Formats Text --> PrepareTextMsgL[Prepare TextSendMessage]
        PrepareTextMsgL -- _send_line_message --> SendHelperL[_send_line_message]

        ImgL -- send_images? --> UploadS3L[_upload_image_to_s3]
        UploadS3L -- Success (URL) --> PrepareImageMsgL[Prepare ImageSendMessage]
        PrepareImageMsgL -- _send_line_message --> SendHelperL
        UploadS3L -- Fail/Timeout --> LogWarnL[Log Warning, Send Nothing]

        SendHelperL -- line_bot_api.multicast()/push() --> LineAPI{LINE Messaging API}
        LineAPI -- Response --> HandleRespL{Handle Response/Error}
        HandleRespL -- Retry? --> SendHelperL
    end

    style LineAPI fill:#00B900,stroke:#FFF,stroke-width:2px
    style UploadS3L fill:#FF9900,stroke:#232F3E,stroke-width:1px
```

## Combined Next Steps (Updated)

1.  Implement S3 image upload functionality in `rsudp/c_googlechat.py` and `rsudp/c_line.py`.
2.  Update `rsudp/c_settings.py` to include S3 related configurations for `googlechat` and `line`.
3.  Update `rsudp/client.py` to pass S3 configurations to `GoogleChatter` and `LINENotifier`.
4.  Update `rsudp/test.py`'s `make_test_settings` to include S3 test configurations for `googlechat` and `line`.
5.  Update documentation.