import sys
import os # For os.path.basename
import time
import requests  # Dependency: Make sure 'requests' is installed
from datetime import timezone, timedelta # For JST conversion
import boto3 # For S3 upload
from botocore.exceptions import NoCredentialsError, PartialCredentialsError, ClientError
from botocore.config import Config as BotoConfig # For S3 timeout

from rsudp import printM, printW, printE, helpers
import rsudp.raspberryshake as rs
from rsudp.test import TEST

class GoogleChatter(rs.ConsumerThread):
    """
    Consumer thread to send seismic event notifications to Google Chat via Incoming Webhook.

    Inherits from rsudp.raspberryshake.ConsumerThread.

    Args:
        webhook_url (str): The Google Chat Incoming Webhook URL.
        q (queue.Queue): The queue object for receiving messages from the producer.
        extra_text (str, optional): Additional text to append to the notification message. Defaults to False.
        testing (bool, optional): If True, runs in testing mode without actually sending messages. Defaults to False.
    """
    def __init__(self, webhook_url, q=False, extra_text=False, testing=False,
                 send_images=False, s3_bucket_name=None, s3_object_key_prefix=None,
                 s3_aws_region=None, s3_upload_timeout_seconds=3,
                 aws_access_key_id=None, aws_secret_access_key=None): # Added S3 params
        super().__init__()
        if not q:
            printE('GoogleChatter: Queue is required.', self.sender)
            raise ValueError("Queue object is required for GoogleChatter")
        if not webhook_url:
            printE('GoogleChatter: Webhook URL is required.', self.sender)
            raise ValueError("Webhook URL is required for GoogleChatter")

        self.queue = q
        self.sender = 'GoogleChat'
        self.alive = True
        self.webhook_url = webhook_url
        self.testing = testing
        self.fmt = '%Y-%m-%d %H:%M:%S.%f' # Original format for parsing
        self.jst_fmt = '%Y年%m月%d日 %H時%M分%S秒 (JST)' # JST format for display
        self.region_text = f"{rs.region.title()}地方" if rs.region else "不明な地域"
        # Resolve extra text, ensuring it fits within potential Google Chat limits (though usually generous for text)
        self.extra_text = helpers.resolve_extra_text(extra_text, max_len=4000, sender=self.sender) # Google Chat limit is large

        self.livelink = f'https://stationview.raspberryshake.org/#?net={rs.net}&sta={rs.stn}' # Removed "live feed ➡️" for cleaner link
        self.last_message = False

        # S3 Image Settings
        self.send_images = send_images
        self.s3_bucket_name = s3_bucket_name
        self.s3_object_key_prefix = s3_object_key_prefix if s3_object_key_prefix else "rsudp/googlechat/"
        self.s3_aws_region = s3_aws_region
        self.s3_upload_timeout_seconds = s3_upload_timeout_seconds
        self.s3_client = None

        if self.send_images and self.s3_bucket_name:
            try:
                boto_config = BotoConfig(
                    connect_timeout=self.s3_upload_timeout_seconds,
                    read_timeout=self.s3_upload_timeout_seconds,
                    retries={'max_attempts': 1} # Attempt only once for quick timeout
                )
                if aws_access_key_id and aws_secret_access_key:
                    printM(f"Initializing Boto3 S3 client with provided credentials for region {self.s3_aws_region or 'default'}.", self.sender)
                    self.s3_client = boto3.client(
                        's3',
                        aws_access_key_id=aws_access_key_id,
                        aws_secret_access_key=aws_secret_access_key,
                        region_name=self.s3_aws_region, # Can be None, Boto3 will try to determine
                        config=boto_config
                    )
                else:
                    printM(f"Initializing Boto3 S3 client using default credential chain for region {self.s3_aws_region or 'default'}.", self.sender)
                    self.s3_client = boto3.client('s3', region_name=self.s3_aws_region, config=boto_config)
                printM("Boto3 S3 client initialized for GoogleChatter.", self.sender)
            except (NoCredentialsError, PartialCredentialsError):
                printE("AWS credentials not found for S3. Image sending will be disabled.", self.sender)
                self.s3_client = None
            except ClientError as e:
                printE(f"AWS ClientError during S3 client initialization: {e}. Image sending will be disabled.", self.sender)
                self.s3_client = None
            except Exception as e:
                printE(f"Unexpected error during S3 client initialization: {e}. Image sending will be disabled.", self.sender)
                self.s3_client = None
        elif self.send_images and not self.s3_bucket_name:
            printW("send_images is true but s3_bucket_name is not configured. Image sending will be disabled.", self.sender)
            self.send_images = False


        printM('Starting.', self.sender)

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

    def _send_message(self, message):
        """Sends the formatted message to the Google Chat webhook."""
        payload = {'text': message}
        response = None
        try:
            printM(f'Sending message to Google Chat: {message}', sender=self.sender)
            if not self.testing:
                response = requests.post(self.webhook_url, json=payload, timeout=10) # Added timeout
                response.raise_for_status() # Raise HTTPError for bad responses (4xx or 5xx)
                printM('Message sent successfully.', sender=self.sender)
            else:
                printM('Testing mode: Message prepared but not sent.', sender=self.sender)
                TEST['c_googlechat'][1] = True # Assuming a test key exists
                # You might need to add 'c_googlechat' to rsudp.test.TEST dictionary
                # Example: TEST = {'c_googlechat': ['Google Chat message sending', False], ...}

        except requests.exceptions.RequestException as e:
            printE(f'Could not send message - {e}', sender=self.sender)
            # Implement retry logic if desired
            printE('Attempting retry in 5 seconds...', sender=self.sender, spaces=True)
            time.sleep(5)
            try:
                if not self.testing:
                    response = requests.post(self.webhook_url, json=payload, timeout=15) # Longer timeout for retry
                    response.raise_for_status()
                    printM('Message sent successfully on retry.', sender=self.sender)
                else:
                     printM('Testing mode: Retry prepared but not sent.', sender=self.sender)
                     TEST['c_googlechat'][1] = True # Mark as success in testing on retry attempt
            except requests.exceptions.RequestException as e2:
                printE(f'Final failure to send message - {e2}', sender=self.sender)
                if self.testing:
                    TEST['c_googlechat'][1] = False # Mark as failure in testing

        self.last_message = message


    def _when_alarm(self, d):
        """Actions to take when an ALARM message is received."""
        event_time_utc_obspy = helpers.fsec(helpers.get_msg_time(d))
        
        # Convert to JST
        py_datetime_utc = event_time_utc_obspy.datetime
        jst_tz = timezone(timedelta(hours=9))
        event_time_jst = py_datetime_utc.astimezone(jst_tz)
        jst_time_str = event_time_jst.strftime(self.jst_fmt)

        message_parts = [
            f"【地震イベント検知】",
            f"発生時刻: {jst_time_str}\n",
            f"詳細は以下のリンクをご確認ください:",
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
        
        message = "\n".join(message_parts)

        self._send_message(message)

    def _upload_image_to_s3(self, local_file_path, object_key_name):
        """Uploads an image to S3 and returns its public URL."""
        if not self.s3_client:
            printE("S3 client not initialized, cannot upload image.", self.sender)
            return None
        try:
            printM(f"Uploading {local_file_path} to S3 bucket {self.s3_bucket_name} as {object_key_name}", self.sender)
            self.s3_client.upload_file(local_file_path, self.s3_bucket_name, object_key_name, ExtraArgs={'ACL': 'public-read'})
            # Construct public URL (common S3 URL format, adjust if using custom domain or different region format)
            # For virtual-hosted style: https://bucket-name.s3.Region.amazonaws.com/key-name
            # For path-style: https://s3.Region.amazonaws.com/bucket-name/key-name
            # Assuming virtual-hosted style is common. Region might not be needed if bucket name is globally unique and accessed via s3.amazonaws.com
            # However, explicitly including region is safer.
            s3_region_for_url = self.s3_aws_region or self.s3_client.meta.region_name # Get region if not explicitly set
            if not s3_region_for_url: # Fallback if region still not determined (e.g. global endpoint)
                 printW("S3 region not determined, URL might be incorrect. Assuming us-east-1 for URL or using path-style if possible.", self.sender)
                 # Path style might be more robust if region is tricky
                 # return f"https://s3.amazonaws.com/{self.s3_bucket_name}/{object_key_name}"
                 s3_region_for_url = "us-east-1" # Defaulting, this might need adjustment

            # Check if bucket name contains dots, which affects virtual-hosted style URL
            if '.' in self.s3_bucket_name:
                 # Path-style URL for buckets with dots
                 public_url = f"https://s3-{s3_region_for_url}.amazonaws.com/{self.s3_bucket_name}/{object_key_name}"
            else:
                 # Virtual-hosted style URL
                 public_url = f"https://{self.s3_bucket_name}.s3.{s3_region_for_url}.amazonaws.com/{object_key_name}"

            printM(f"Image uploaded to S3: {public_url}", self.sender)
            return public_url
        except FileNotFoundError:
            printE(f"Local image file not found: {local_file_path}", self.sender)
            return None
        except NoCredentialsError:
            printE("AWS credentials not found for S3 upload.", self.sender)
            return None
        except PartialCredentialsError:
            printE("Incomplete AWS credentials for S3 upload.", self.sender)
            return None
        except ClientError as e:
            if e.response['Error']['Code'] == 'ExpiredToken':
                printE("AWS token has expired. Cannot upload to S3.", self.sender)
            elif e.response['Error']['Code'] == 'AccessDenied':
                printE(f"Access Denied for S3 upload to {self.s3_bucket_name}/{object_key_name}. Check IAM permissions.", self.sender)
            else:
                printE(f"S3 ClientError during upload: {e}", self.sender)
            return None
        except Exception as e: # Catch any other exception during upload, including potential timeouts from BotoConfig
            printE(f"Failed to upload image to S3: {e}", self.sender)
            return None

    def _when_img(self, d):
        """Actions to take when an IMGPATH message is received."""
        if not self.send_images or not self.s3_client:
            if self.send_images and not self.s3_client:
                printW("Image sending enabled but S3 client not available. Skipping image for Google Chat.", self.sender)
            else: # send_images is false
                printM('Ignoring IMGPATH message (image sending disabled for Google Chat).', self.sender)
            return

        local_image_path = helpers.get_msg_path(d)
        if not os.path.exists(local_image_path):
            printW(f'Could not find image for Google Chat: {local_image_path}', self.sender)
            return

        object_key = self.s3_object_key_prefix + os.path.basename(local_image_path)
        
        # Attempt to upload to S3
        s3_image_url = self._upload_image_to_s3(local_image_path, object_key)

        if s3_image_url:
            event_time_utc_obspy = helpers.fsec(helpers.get_msg_time(d))
            py_datetime_utc = event_time_utc_obspy.datetime
            jst_tz = timezone(timedelta(hours=9))
            event_time_jst = py_datetime_utc.astimezone(jst_tz)
            jst_time_str = event_time_jst.strftime(self.jst_fmt)

            message_parts = [
                f"【地震イベント画像】",
                f"発生時刻: {jst_time_str}",
                f"画像: {s3_image_url}\n",
                f"詳細は以下のリンクをご確認ください:",
                f"{self.livelink}"
            ]
            if self.extra_text:
                message_parts.append(f"\n{str(self.extra_text).strip()}")
            
            message_parts.extend([
                "\n---",
                "観測点情報:",
                f"ステーションID: {rs.net}.{rs.stn}",
                f"地域: {self.region_text}"
            ])
            message = "\n".join(message_parts)
            self._send_message(message)
            TEST['c_googlechatimg'][1] = True # Assuming a test key for image success
        else:
            printW(f"Failed to upload image {local_image_path} to S3. Sending Google Chat message without image.", self.sender)
            TEST['c_googlechatimg'][1] = False # Mark as S3 image failure

    def run(self):
        """Main loop to process messages from the queue."""
        while self.alive:
            d = self.getq()

            if 'ALARM' in str(d):
                self._when_alarm(d)

            elif 'IMGPATH' in str(d):
                self._when_img(d) # Call _when_img to handle image sending

# Example of how to add to rsudp.test.TEST (in rsudp/test.py)
# TEST = {
#     ... existing tests ...
#     'c_googlechat': ['Google Chat message sending', False],
# }