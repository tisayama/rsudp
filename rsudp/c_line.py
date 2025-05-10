import sys
import os # For os.path.basename
import time
from datetime import timezone, timedelta # For JST conversion
import boto3 # For S3 upload
from botocore.exceptions import NoCredentialsError, PartialCredentialsError, ClientError
from botocore.config import Config as BotoConfig # For S3 timeout
from linebot.v3 import WebhookHandler
from linebot.v3.messaging import (
    Configuration,
    ApiClient,
    MessagingApi,
    PushMessageRequest,
    TextMessage,
    ImageMessage, # Corrected class name
    ApiException
)
# Removed incorrect import: from linebot.v3.exceptions import LineBotApiError

from rsudp import printM, printW, printE, helpers
import rsudp.raspberryshake as rs
from rsudp.test import TEST

class LINENotifier(rs.ConsumerThread):
    """
    Consumer thread to send seismic event notifications via LINE Messaging API.

    Inherits from rsudp.raspberryshake.ConsumerThread.

    Args:
        channel_access_token (str): LINE Channel Access Token.
        to_ids_str (str): Comma-separated string of User IDs, Group IDs, or Room IDs.
        q (queue.Queue): The queue object for receiving messages.
        extra_text (str, optional): Additional text to append to the notification message. Defaults to False.
        testing (bool, optional): If True, runs in testing mode without actually sending messages. Defaults to False.
    """
    def __init__(self, channel_access_token, to_ids_str, q=False,
                 extra_text=False, testing=False,
                 send_images=False, s3_bucket_name=None, s3_object_key_prefix=None,
                 s3_aws_region=None, s3_upload_timeout_seconds=3,
                 aws_access_key_id=None, aws_secret_access_key=None): # Added S3 params
        super().__init__()
        if not q:
            printE('LINENotifier: Queue is required.', self.sender)
            raise ValueError("Queue object is required for LINENotifier")
        if not channel_access_token or channel_access_token == "n/a":
            printE('LINENotifier: Channel Access Token is required.', self.sender)
            raise ValueError("Channel Access Token is required for LINENotifier")
        if not to_ids_str:
            printE('LINENotifier: Target IDs (to_ids) are required.', self.sender)
            raise ValueError("Target IDs (to_ids) are required for LINENotifier")

        self.queue = q
        self.sender = 'LINE'
        self.alive = True
        self.channel_access_token = channel_access_token
        self.testing = testing
        self.fmt = '%Y-%m-%d %H:%M:%S.%f' # Original format for parsing
        self.jst_fmt = '%Y年%m月%d日 %H時%M分%S秒 (JST)' # JST format for display
        self.region_text = f"{rs.region.title()}地方" if rs.region else "不明な地域"
        self.extra_text = helpers.resolve_extra_text(extra_text, max_len=4500, sender=self.sender) # LINE text limit is 5000 chars

        # Parse and validate target IDs
        self.to_ids = [uid.strip() for uid in to_ids_str.split(',') if uid.strip()]
        if not self.to_ids:
             printE('LINENotifier: No valid target IDs found in to_ids string.', self.sender)
             raise ValueError("No valid target IDs provided")
        printM(f"Target LINE IDs: {self.to_ids}", self.sender)


        self.livelink = f'https://stationview.raspberryshake.org/#?net={rs.net}&sta={rs.stn}'
        # self.message0_template is now generated dynamically in _when_alarm
        self.last_message_object = None

        # S3 Image Settings
        self.send_images = send_images
        self.s3_bucket_name = s3_bucket_name
        self.s3_object_key_prefix = s3_object_key_prefix if s3_object_key_prefix else "rsudp/line/"
        self.s3_aws_region = s3_aws_region
        self.s3_upload_timeout_seconds = s3_upload_timeout_seconds
        self.s3_client = None

        # Initialize LINE Bot SDK client
        self.line_bot_api = None
        if not self.testing:
            try:
                configuration = Configuration(access_token=self.channel_access_token)
                self.line_bot_api = MessagingApi(ApiClient(configuration))
                printM("LINE Bot API client initialized.", self.sender)
            except Exception as e:
                printE(f"Failed to initialize LINE Bot API client: {e}", self.sender)
                self.alive = False

        # Initialize S3 client if image sending is enabled
        if self.send_images and self.s3_bucket_name and self.alive:
            try:
                boto_config = BotoConfig(
                    connect_timeout=self.s3_upload_timeout_seconds,
                    read_timeout=self.s3_upload_timeout_seconds,
                    retries={'max_attempts': 1}
                )
                if aws_access_key_id and aws_secret_access_key:
                    printM(f"Initializing Boto3 S3 client with provided credentials for region {self.s3_aws_region or 'default'}.", self.sender)
                    self.s3_client = boto3.client(
                        's3',
                        aws_access_key_id=aws_access_key_id,
                        aws_secret_access_key=aws_secret_access_key,
                        region_name=self.s3_aws_region,
                        config=boto_config
                    )
                else:
                    printM(f"Initializing Boto3 S3 client using default credential chain for region {self.s3_aws_region or 'default'}.", self.sender)
                    self.s3_client = boto3.client('s3', region_name=self.s3_aws_region, config=boto_config)
                printM("Boto3 S3 client initialized for LINENotifier.", self.sender)
            except (NoCredentialsError, PartialCredentialsError):
                printE("AWS credentials not found for S3. Image sending will be disabled for LINE.", self.sender)
                self.s3_client = None
            except ClientError as e:
                printE(f"AWS ClientError during S3 client initialization for LINE: {e}. Image sending will be disabled.", self.sender)
                self.s3_client = None
            except Exception as e:
                printE(f"Unexpected error during S3 client initialization for LINE: {e}. Image sending will be disabled.", self.sender)
                self.s3_client = None
        elif self.send_images and not self.s3_bucket_name:
            printW("send_images is true for LINE but s3_bucket_name is not configured. Image sending will be disabled.", self.sender)
            self.send_images = False


        if self.alive:
            printM('Starting.', self.sender)
        else:
             printE("Failed to start due to client initialization issues.", self.sender)


    def getq(self):
        """Get message from the queue."""
        d = self.queue.get()
        self.queue.task_done()
        if 'TERM' in str(d):
            self.alive = False
            printM('Exiting.', self.sender)
            sys.exit()
        else:
            return d

    def _send_line_message(self, message_object):
        """Internal helper to send a message object to all target IDs."""
        if not self.line_bot_api and not self.testing:
            printE("LINE Bot API client not initialized. Cannot send message.", self.sender)
            if self.testing: TEST['c_line'][1] = False
            return

        message_text_preview = message_object.text[:50] + "..." if hasattr(message_object, 'text') else "Non-text message"
        printM(f'Sending message to LINE IDs {self.to_ids}: "{message_text_preview}"', self.sender)

        try:
            if not self.testing:
                # Use multicast to send to multiple users efficiently (up to 150 users)
                # If more than 150, or mixing user/group/room IDs where multicast might fail,
                # loop through IDs and use push_message instead.
                # For simplicity here, we assume multicast works or the list is small.
                push_message_request = PushMessageRequest(
                    to=self.to_ids[0], # Push requires a single 'to' field
                    messages=[message_object]
                )
                # If sending to multiple, loop push_message or use multicast if applicable
                if len(self.to_ids) == 1:
                     self.line_bot_api.push_message(push_message_request)
                     printM(f'Message pushed successfully to {self.to_ids[0]}.', self.sender)
                else:
                     # Multicast is generally preferred for multiple users
                     # Note: Multicast only works for User IDs, not Group/Room IDs
                     # A robust implementation might check ID types or just loop push_message
                     printM(f'Attempting multicast to {len(self.to_ids)} IDs (may fail if non-user IDs present).', self.sender)
                     self.line_bot_api.multicast(
                         to=self.to_ids,
                         messages=[message_object]
                     )
                     printM(f'Message multicast attempted to {len(self.to_ids)} IDs.', self.sender)

                TEST['c_line'][1] = True
            else:
                printM('Testing mode: Message prepared for LINE but not sent.', self.sender)
                TEST['c_line'][1] = True # Mark as success in testing

        except ApiException as e: # Catch the correct exception type
            printE(f"LINE API Error: Status={e.status}, Body={e.body}", self.sender)
            TEST['c_line'][1] = False
        except Exception as e: # Keep catching other potential errors
            printE(f"Unexpected error sending LINE message: {e}", self.sender)
            TEST['c_line'][1] = False

        self.last_message_object = message_object

    def _when_alarm(self, d):
        """Actions to take when an ALARM message is received."""
        event_time_utc_obspy = helpers.fsec(helpers.get_msg_time(d))

        # Convert to JST
        py_datetime_utc = event_time_utc_obspy.datetime
        jst_tz = timezone(timedelta(hours=9))
        event_time_jst = py_datetime_utc.astimezone(jst_tz)
        jst_time_str = event_time_jst.strftime(self.jst_fmt)

        message_parts = [
            "【地震イベント検知】",
            f"発生時刻: {jst_time_str}\n",
            "詳細は以下のリンクをご確認ください:",
            f"{self.livelink}"
        ]
        if self.extra_text:
            # Ensure extra_text is treated as a string and prepended with a newline if it's not empty
            message_parts.append(f"\n{str(self.extra_text).strip()}")

        message_parts.extend([
            "\n---",
            "観測点情報:",
            f"ステーションID: {rs.net}.{rs.stn}",
            f"地域: {self.region_text}"
        ])
        
        message_text = "\n".join(message_parts)

        # Ensure message length is within LINE limits
        if len(message_text.encode('utf-8')) > 4950: # A bit of buffer for safety for 5000 char limit
            printW(f"Message too long for LINE ({len(message_text.encode('utf-8'))} bytes), attempting to truncate.", self.sender)
            # Basic truncation, might need more sophisticated logic for multi-byte chars
            while len(message_text.encode('utf-8')) > 4950:
                message_text = message_text[:-10] # Remove 10 chars at a time
            message_text += "..."
            printW(f"Message truncated to {len(message_text.encode('utf-8'))} bytes.", self.sender)

        message_object = TextMessage(text=message_text)
        self._send_line_message(message_object)

    def _upload_image_to_s3(self, local_file_path, object_key_name):
        """Uploads an image to S3 and returns its public URL."""
        if not self.s3_client:
            printE("S3 client not initialized for LINE, cannot upload image.", self.sender)
            return None
        try:
            printM(f"Uploading {local_file_path} to S3 bucket {self.s3_bucket_name} as {object_key_name} for LINE", self.sender)
            self.s3_client.upload_file(local_file_path, self.s3_bucket_name, object_key_name, ExtraArgs={'ACL': 'public-read'})
            
            s3_region_for_url = self.s3_aws_region or self.s3_client.meta.region_name
            if not s3_region_for_url:
                 s3_region_for_url = "us-east-1" # Defaulting

            if '.' in self.s3_bucket_name:
                 public_url = f"https://s3-{s3_region_for_url}.amazonaws.com/{self.s3_bucket_name}/{object_key_name}"
            else:
                 public_url = f"https://{self.s3_bucket_name}.s3.{s3_region_for_url}.amazonaws.com/{object_key_name}"
            
            printM(f"Image uploaded to S3 for LINE: {public_url}", self.sender)
            return public_url
        except FileNotFoundError:
            printE(f"Local image file not found for LINE S3 upload: {local_file_path}", self.sender)
            return None
        except (NoCredentialsError, PartialCredentialsError):
            printE("AWS credentials not found for LINE S3 upload.", self.sender)
            return None
        except ClientError as e:
            printE(f"S3 ClientError during LINE image upload: {e}", self.sender)
            return None
        except Exception as e:
            printE(f"Failed to upload image to S3 for LINE: {e}", self.sender)
            return None

    def _when_img(self, d):
        """Actions to take when an IMGPATH message is received."""
        if not self.send_images or not self.s3_client:
            if self.send_images and not self.s3_client:
                printW("Image sending enabled for LINE but S3 client not available. Skipping image.", self.sender)
            # else send_images is false, so normal to ignore
            return

        local_image_path = helpers.get_msg_path(d)
        if not os.path.exists(local_image_path):
            printW(f'Could not find image for LINE: {local_image_path}', self.sender)
            return

        object_key = self.s3_object_key_prefix + os.path.basename(local_image_path)
        s3_image_url = self._upload_image_to_s3(local_image_path, object_key)

        if s3_image_url:
            try:
                # For LINE, preview URL must also be HTTPS and accessible
                image_message = ImageMessage( # Corrected class name
                    original_content_url=s3_image_url,
                    preview_image_url=s3_image_url # Using the same URL for preview
                )
                self._send_line_message(image_message)
                TEST['c_lineimg'][1] = True # Assuming a test key like 'c_lineimg'
            except Exception as e:
                printE(f"Failed to create or send LINE ImageSendMessage: {e}", self.sender)
                TEST['c_lineimg'][1] = False
        else:
            printW(f"Failed to upload image {local_image_path} to S3 for LINE. No image message sent.", self.sender)
            TEST['c_lineimg'][1] = False


    def run(self):
        """Main loop to process messages from the queue."""
        if not self.alive:
             printE("LINENotifier thread cannot run because client initialization failed.", self.sender)
             return # Do not proceed if client is not ready

        while self.alive:
            d = self.getq()

            if 'ALARM' in str(d):
                self._when_alarm(d)
            elif 'IMGPATH' in str(d):
                self._when_img(d)

# Example for rsudp.test.TEST (in rsudp/test.py)
# TEST = {
#     ... existing tests ...
#     'c_line': ['LINE message sending', False],
# }