import sys
import time
import json # Though not strictly needed for simple string messages, good to have if future enhancements use JSON
import boto3 # AWS SDK for Python
from botocore.exceptions import NoCredentialsError, PartialCredentialsError, ClientError

from rsudp import printM, printW, printE, helpers
import rsudp.raspberryshake as rs
from rsudp.test import TEST

class SNSNotifier(rs.ConsumerThread):
    """
    Consumer thread to send seismic event notifications to Amazon SNS.

    Inherits from rsudp.raspberryshake.ConsumerThread.

    Args:
        topic_arn (str): The Amazon SNS topic ARN to publish messages to.
        q (queue.Queue): The queue object for receiving messages.
        aws_access_key_id (str, optional): AWS Access Key ID. If None, Boto3 will try other credential sources.
        aws_secret_access_key (str, optional): AWS Secret Access Key. If None, Boto3 will try other credential sources.
        aws_region (str): The AWS region where the SNS topic resides (e.g., 'us-east-1').
        extra_text (str, optional): Additional text to append to the notification message. Defaults to False.
        testing (bool, optional): If True, runs in testing mode without actually sending messages. Defaults to False.
    """
    def __init__(self, topic_arn, q=False,
                 aws_access_key_id=None, aws_secret_access_key=None, aws_region=None,
                 extra_text=False, testing=False):
        super().__init__()
        if not q:
            printE('SNSNotifier: Queue is required.', self.sender)
            raise ValueError("Queue object is required for SNSNotifier")
        if not topic_arn:
            printE('SNSNotifier: Topic ARN is required.', self.sender)
            raise ValueError("Topic ARN is required for SNSNotifier")
        if not aws_region:
            printE('SNSNotifier: AWS Region is required.', self.sender)
            raise ValueError("AWS Region is required for SNSNotifier")

        self.queue = q
        self.sender = 'SNSNotifier'
        self.alive = True
        self.topic_arn = topic_arn
        self.aws_region = aws_region
        self.testing = testing
        self.fmt = '%Y-%m-%d %H:%M:%S.%f'
        self.region_info = f' - region: {rs.region.title()}' if rs.region else ''
        self.extra_text = helpers.resolve_extra_text(extra_text, max_len=30000, sender=self.sender) # SNS message size limit is 256KB

        self.livelink = f'https://stationview.raspberryshake.org/#?net={rs.net}&sta={rs.stn}'
        self.message0_template = f'(Raspberry Shake station {rs.net}.{rs.stn}{self.region_info}) Event detected at'
        self.last_message_body = None

        try:
            if aws_access_key_id and aws_secret_access_key:
                printM(f"Initializing Boto3 SNS client with provided credentials for region {self.aws_region}.", self.sender)
                self.sns_client = boto3.client(
                    'sns',
                    aws_access_key_id=aws_access_key_id,
                    aws_secret_access_key=aws_secret_access_key,
                    region_name=self.aws_region
                )
            else:
                printM(f"Initializing Boto3 SNS client using default credential chain for region {self.aws_region}.", self.sender)
                self.sns_client = boto3.client('sns', region_name=self.aws_region)
            # Test credentials by making a simple call (optional, but good for early feedback)
            # self.sns_client.list_topics() # This would require ListTopics permission
            printM("Boto3 SNS client initialized.", self.sender)
        except (NoCredentialsError, PartialCredentialsError):
            printE("AWS credentials not found. Please configure credentials (e.g., via aws configure, environment variables, or in settings).", self.sender)
            self.sns_client = None # Ensure client is None if init fails
            self.alive = False # Prevent thread from running without a client
        except ClientError as e:
            printE(f"AWS ClientError during SNS client initialization: {e}", self.sender)
            self.sns_client = None
            self.alive = False
        except Exception as e:
            printE(f"Unexpected error during SNS client initialization: {e}", self.sender)
            self.sns_client = None
            self.alive = False

        if self.alive:
            printM('Starting.', self.sender)
        else:
            printE("Failed to start due to SNS client initialization issues.", self.sender)


    def getq(self):
        """Get message from the queue."""
        d = self.queue.get()
        self.queue.task_done()
        if 'TERM' in str(d):
            self.alive = False
            printM('Exiting.', self.sender)
            if self.sns_client: # Clean up client if it exists
                # Boto3 clients don't typically need explicit closing unless using sessions in a specific way
                pass
            sys.exit()
        else:
            return d

    def _send_sns_message(self, message_body):
        """Internal helper to publish a message to the SNS topic."""
        if not self.sns_client:
            printE("SNS client not initialized. Cannot send message.", self.sender)
            if self.testing: TEST['c_sns'][1] = False
            return

        try:
            printM(f'Publishing message to SNS Topic {self.topic_arn}: "{message_body}"', self.sender)
            if not self.testing:
                response = self.sns_client.publish(
                    TopicArn=self.topic_arn,
                    Message=message_body,
                    # MessageStructure='string' # Default is string, so not strictly needed
                )
                printM(f'Message published successfully to SNS. Message ID: {response.get("MessageId")}', self.sender)
                TEST['c_sns'][1] = True
            else:
                printM('Testing mode: Message prepared for SNS but not sent.', self.sender)
                TEST['c_sns'][1] = True # Mark as success in testing

        except ClientError as e:
            error_code = e.response.get("Error", {}).get("Code")
            if error_code == "InvalidParameterValue":
                printE(f"SNS Publish Error: Invalid parameter value. Check Topic ARN and message content. Details: {e}", self.sender)
            elif error_code == "AuthorizationError":
                printE(f"SNS Publish Error: Authorization error. Check IAM permissions for sns:Publish on topic {self.topic_arn}. Details: {e}", self.sender)
            elif error_code == "NotFound":
                 printE(f"SNS Publish Error: Topic not found. Ensure Topic ARN '{self.topic_arn}' is correct and exists in region '{self.aws_region}'. Details: {e}", self.sender)
            else:
                printE(f"SNS Publish Error: {e}", self.sender)
            TEST['c_sns'][1] = False
            # No retry implemented here, but could be added for certain error types
        except Exception as e:
            printE(f"Unexpected error publishing to SNS: {e}", self.sender)
            TEST['c_sns'][1] = False

        self.last_message_body = message_body

    def _when_alarm(self, d):
        """Actions to take when an ALARM message is received."""
        event_time = helpers.fsec(helpers.get_msg_time(d))
        last_event_str = f'{event_time.strftime(self.fmt)[:22]}'
        
        message_body = f"{self.message0_template} {last_event_str} UTC.\n"
        if self.extra_text:
            message_body += f"{self.extra_text}\n"
        message_body += f"Live Feed: {self.livelink}"

        self._send_sns_message(message_body)

    def _when_img(self, d):
        """Image handling - SNS does not directly support image attachments in basic publish."""
        printM('Ignoring IMGPATH message (image sending not supported by this SNS notifier).', self.sender)
        # If a link to an image were available (e.g., uploaded elsewhere), it could be included in an ALARM message.
        pass

    def run(self):
        """Main loop to process messages from the queue."""
        if not self.alive or not self.sns_client:
            printE("SNSNotifier thread cannot run because client initialization failed.", self.sender)
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
#     'c_sns': ['SNS message publishing', False],
# }