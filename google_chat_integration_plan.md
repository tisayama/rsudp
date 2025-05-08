# RSUDP Notification Services Integration Plan

This document outlines the plan for integrating Google Chat, Discord, Amazon SNS, and LINE notifications into RSUDP.

## Objective

Create new consumer threads that send notifications about seismic events detected by RSUDP to specified Google Chat spaces, Discord channels, Amazon SNS topics, and LINE users/groups.

## Design: Google Chat

Based on the existing `c_telegram.py` and `c_tweet.py` modules, the new `GoogleChatter` class will be implemented as follows:

*   **Class Name:** `GoogleChatter` (inherits from `rsudp.raspberryshake.ConsumerThread`)
*   **File Name:** `rsudp/c_googlechat.py`
*   **Dependencies:** `requests` library
*   **Configuration:** Requires a `googlechat_webhook_url` setting in the RSUDP configuration.

### Key Methods and Functionality (Google Chat)

*   **`__init__(self, webhook_url, q=False, extra_text=False, testing=False)`**:
    *   Initializes the consumer with the Webhook URL, message queue, optional extra text, and testing flag.
    *   Sets up necessary attributes like message format, station info, and sender name (`GoogleChat`).
*   **`getq(self)`**:
    *   Retrieves messages from the queue, handling the `TERM` signal for graceful shutdown. (Similar to existing consumers).
*   **`_when_alarm(self, d)`**:
    *   Triggered by `ALARM` messages.
    *   Formats a simple text message containing event details.
    *   Creates a JSON payload in the format `{'text': formatted_message}`.
    *   Sends the payload to the configured `webhook_url` using `requests.post()`.
    *   Includes error handling and a retry mechanism.
*   **`_when_img(self, d)`**:
    *   Ignores `IMGPATH` messages (image sending disabled for Google Chat).
*   **`run(self)`**:
    *   Main loop, calls `_when_alarm` for `ALARM` messages, ignores `IMGPATH`.

### Workflow Diagram (Google Chat - Mermaid)

```mermaid
graph TD
    subgraph RSUDP Core
        Producer[p_producer] -- Data --> Consumer[c_consumer]
    end

    subgraph GoogleChatter Module (c_googlechat.py)
        Consumer -- Queue --> GC(GoogleChatter)
        GC -- Reads Queue --> MsgType{Message Type?}
        MsgType -- ALARM --> Alarm[_when_alarm]
        MsgType -- IMGPATH --> IgnoreImg[Ignore IMGPATH]
        MsgType -- TERM --> Terminate[Exit Thread]

        Alarm -- Formats Text --> PrepareJsonText[Prepare JSON (Text)]
        PrepareJsonText -- requests.post --> GoogleAPI{Google Chat API (Webhook)}

        GoogleAPI -- Response --> HandleResp{Handle Response/Error}
        HandleResp -- Retry? --> PrepareJsonText
    end

    style GoogleAPI fill:#f9f,stroke:#333,stroke-width:2px
```

## Design: Discord

Similar to Google Chat, a new `Discorder` class will be implemented:

*   **Class Name:** `Discorder` (inherits from `rsudp.raspberryshake.ConsumerThread`)
*   **File Name:** `rsudp/c_discord.py`
*   **Dependencies:** `requests` library
*   **Configuration:** Requires `discord_webhook_url`, `discord_use_embed`, `discord_send_images`, `discord_extra_text` settings.

### Key Methods and Functionality (Discord)

*   **`__init__(self, webhook_url, q=False, use_embed=True, send_images=True, extra_text=False, testing=False)`**:
    *   Initializes with Discord Webhook URL, queue, embed/image flags, extra text, and testing flag.
    *   Sets up sender name (`Discord`).
*   **`getq(self)`**: Similar to other consumers.
*   **`_send_message(self, payload=None, files=None)`**: Internal helper to send messages/files via `requests.post`. Handles errors and retries.
*   **`_when_alarm(self, d)`**:
    *   Triggered by `ALARM` messages.
    *   Formats a Discord Embed object containing event details (timestamp, station, region, link, extra text).
    *   Creates a JSON payload with the `embeds` field.
    *   Calls `_send_message(payload=payload)` to send.
*   **`_when_img(self, d)`**:
    *   Triggered by `IMGPATH` messages if `send_images` is True.
    *   Opens the image file specified by the path.
    *   Prepares the `files` argument for `requests.post` (`{'file': (filename, file_data)}`).
    *   Optionally creates a simple payload (e.g., `{'content': 'Event Image'}`).
    *   Calls `_send_message(payload=payload, files=files)` to send the image with optional text/embed.
*   **`run(self)`**: Main loop, calls `_when_alarm` or `_when_img` based on message type.

### Workflow Diagram (Discord - Mermaid)

```mermaid
graph TD
    subgraph RSUDP Core
        Producer[p_producer] -- Data --> Consumer[c_consumer]
    end

    subgraph Discorder Module (c_discord.py)
        Consumer -- Queue --> DC(Discorder)
        DC -- Reads Queue --> MsgType{Message Type?}
        MsgType -- ALARM --> Alarm[_when_alarm]
        MsgType -- IMGPATH --> Img[_when_img]
        MsgType -- TERM --> Terminate[Exit Thread]

        Alarm -- Formats Embed --> PrepareJsonEmbed[Prepare JSON (Embed)]
        PrepareJsonEmbed -- _send_message --> SendHelper[_send_message]

        Img -- send_images? --> PrepareImg[Prepare Image File & Optional Payload]
        PrepareImg -- _send_message --> SendHelper

        SendHelper -- requests.post --> DiscordAPI{Discord API (Webhook)}
        DiscordAPI -- Response --> HandleResp{Handle Response/Error}
        HandleResp -- Retry? --> SendHelper
    end

    style DiscordAPI fill:#7289DA,stroke:#FFF,stroke-width:2px
```

## Design: Amazon SNS

A new `SNSNotifier` class will be implemented:

*   **Class Name:** `SNSNotifier` (inherits from `rsudp.raspberryshake.ConsumerThread`)
*   **File Name:** `rsudp/c_sns.py`
*   **Dependencies:** `boto3` library
*   **Configuration:**
    *   `enabled`: (boolean)
    *   `topic_arn`: (string) SNS Topic ARN.
    *   `aws_access_key_id`: (string, optional)
    *   `aws_secret_access_key`: (string, optional)
    *   `aws_region`: (string) AWS Region.
    *   `extra_text`: (string, optional)

### Key Methods and Functionality (Amazon SNS)

*   **`__init__(self, topic_arn, q=False, aws_access_key_id=None, aws_secret_access_key=None, aws_region=None, extra_text=False, testing=False)`**:
    *   Initializes with SNS Topic ARN, queue, AWS credentials (optional), AWS region, extra text, and testing flag.
    *   Initializes Boto3 SNS client. If credentials are provided in settings, use them; otherwise, Boto3 will use its default credential chain.
    *   Sets up sender name (`SNSNotifier`).
*   **`getq(self)`**: Similar to other consumers.
*   **`_send_sns_message(self, message_body)`**: Internal helper to publish a simple string message to the SNS topic using `boto3_client.publish()`. Handles errors and retries.
*   **`_when_alarm(self, d)`**:
    *   Triggered by `ALARM` messages.
    *   Formats a simple text message containing event details.
    *   Calls `_send_sns_message()` to publish.
*   **`_when_img(self, d)`**:
    *   Ignores `IMGPATH` messages.
*   **`run(self)`**: Main loop, calls `_when_alarm` for `ALARM` messages, ignores `IMGPATH`.

### Workflow Diagram (Amazon SNS - Mermaid)

```mermaid
graph TD
    subgraph RSUDP Core
        Producer[p_producer] -- Data --> Consumer[c_consumer]
    end

    subgraph SNSNotifier Module (c_sns.py)
        Consumer -- Queue --> SN(SNSNotifier)
        SN -- Reads Queue --> MsgType{Message Type?}
        MsgType -- ALARM --> Alarm[_when_alarm]
        MsgType -- IMGPATH --> IgnoreImg[Ignore IMGPATH]
        MsgType -- TERM --> Terminate[Exit Thread]

        Alarm -- Formats Message --> PrepareSnsMsg[Prepare SNS Message Body]
        PrepareSnsMsg -- _send_sns_message --> SendHelper[_send_sns_message]

        SendHelper -- boto3.publish() --> SnsAPI{AWS SNS API}
        SnsAPI -- Response --> HandleResp{Handle Response/Error}
        HandleResp -- Retry? --> SendHelper
    end

    style SnsAPI fill:#FF9900,stroke:#232F3E,stroke-width:2px
```

## Design: LINE

A new `LINENotifier` class will be implemented:

*   **Class Name:** `LINENotifier` (inherits from `rsudp.raspberryshake.ConsumerThread`)
*   **File Name:** `rsudp/c_line.py`
*   **Dependencies:** `line-bot-sdk-python` library
*   **Configuration:**
    *   `enabled`: (boolean)
    *   `channel_access_token`: (string) LINE Channel Access Token.
    *   `to_ids`: (string) Comma-separated list of User IDs, Group IDs, or Room IDs.
    *   `extra_text`: (string, optional)

### Key Methods and Functionality (LINE)

*   **`__init__(self, channel_access_token, to_ids_str, q=False, extra_text=False, testing=False)`**:
    *   Initializes with Channel Access Token, comma-separated `to_ids` string, queue, extra text, and testing flag.
    *   Parses `to_ids_str` into a list of IDs.
    *   Initializes `LineBotApi` client from `line-bot-sdk-python`.
    *   Sets up sender name (`LINE`).
*   **`getq(self)`**: Similar to other consumers.
*   **`_send_line_message(self, message_object)`**: Internal helper to send a LINE message object (e.g., `TextMessage`) to all specified IDs using `line_bot_api.multicast()`. Handles errors (e.g., `LineBotApiError`) and retries.
*   **`_when_alarm(self, d)`**:
    *   Triggered by `ALARM` messages.
    *   Formats a simple text message containing event details.
    *   Creates a `TextMessage` object from `line-bot-sdk-python`.
    *   Calls `_send_line_message()` to send.
*   **`_when_img(self, d)`**:
    *   Ignores `IMGPATH` messages (image sending disabled based on user feedback).
*   **`run(self)`**: Main loop, calls `_when_alarm` for `ALARM` messages, ignores `IMGPATH`.

### Workflow Diagram (LINE - Mermaid)

```mermaid
graph TD
    subgraph RSUDP Core
        Producer[p_producer] -- Data --> Consumer[c_consumer]
    end

    subgraph LINENotifier Module (c_line.py)
        Consumer -- Queue --> LN(LINENotifier)
        LN -- Reads Queue --> MsgType{Message Type?}
        MsgType -- ALARM --> Alarm[_when_alarm]
        MsgType -- IMGPATH --> IgnoreImg[Ignore IMGPATH]
        MsgType -- TERM --> Terminate[Exit Thread]

        Alarm -- Formats Message --> PrepareLineMsg[Prepare TextSendMessage]
        PrepareLineMsg -- _send_line_message --> SendHelper[_send_line_message]

        SendHelper -- line_bot_api.multicast() --> LineAPI{LINE Messaging API}
        LineAPI -- Response --> HandleResp{Handle Response/Error}
        HandleResp -- Retry? --> SendHelper
    end

    style LineAPI fill:#00B900,stroke:#FFF,stroke-width:2px
```

## Combined Next Steps (Updated)

1.  Implement the `GoogleChatter` class in `rsudp/c_googlechat.py` (Done).
2.  Implement the `Discorder` class in `rsudp/c_discord.py` (Done).
3.  Implement the `SNSNotifier` class in `rsudp/c_sns.py` (Done).
4.  Implement the `LINENotifier` class in `rsudp/c_line.py`.
5.  Add `requests` (Done), `boto3` (Done), and `line-bot-sdk-python` to the project's dependencies (`environment.yml`).
6.  Update `rsudp/c_settings.py` and documentation to include settings for `googlechat` (Done), `discord` (Done), `sns` (Done), and `line`.
7.  Update `rsudp/client.py` to conditionally initialize and start `GoogleChatter` (Done), `Discorder` (Done), `SNSNotifier` (Done), and `LINENotifier` based on settings.
8.  Add tests for all new modules in `rsudp/test.py` (Partially done for Google Chat, Discord, SNS. LINE needs adding).
9.  Update `rsudp/test.py`'s `make_test_settings` to include LINE test configurations.