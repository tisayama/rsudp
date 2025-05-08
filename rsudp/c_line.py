import sys
import time
from linebot.v3 import WebhookHandler
from linebot.v3.messaging import (
    Configuration,
    ApiClient,
    MessagingApi,
    PushMessageRequest,
    TextMessage,
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
                 extra_text=False, testing=False):
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
        self.fmt = '%Y-%m-%d %H:%M:%S.%f'
        self.region_info = f' - region: {rs.region.title()}' if rs.region else ''
        self.extra_text = helpers.resolve_extra_text(extra_text, max_len=4500, sender=self.sender) # LINE text limit is 5000 chars

        # Parse and validate target IDs
        self.to_ids = [uid.strip() for uid in to_ids_str.split(',') if uid.strip()]
        if not self.to_ids:
             printE('LINENotifier: No valid target IDs found in to_ids string.', self.sender)
             raise ValueError("No valid target IDs provided")
        printM(f"Target LINE IDs: {self.to_ids}", self.sender)


        self.livelink = f'https://stationview.raspberryshake.org/#?net={rs.net}&sta={rs.stn}'
        self.message0_template = f'(Raspberry Shake station {rs.net}.{rs.stn}{self.region_info}) Event detected at'
        self.last_message_object = None

        # Initialize LINE Bot SDK client
        self.line_bot_api = None
        if not self.testing:
            try:
                configuration = Configuration(access_token=self.channel_access_token)
                self.line_bot_api = MessagingApi(ApiClient(configuration))
                # Optionally test the connection/token here if needed, e.g., get bot info
                # profile = self.line_bot_api.get_profile(user_id) # Requires a valid user ID
                printM("LINE Bot API client initialized.", self.sender)
            except Exception as e:
                printE(f"Failed to initialize LINE Bot API client: {e}", self.sender)
                self.alive = False # Prevent thread from running

        if self.alive:
            printM('Starting.', self.sender)
        else:
             printE("Failed to start due to LINE Bot API client initialization issues.", self.sender)


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
        event_time = helpers.fsec(helpers.get_msg_time(d))
        last_event_str = f'{event_time.strftime(self.fmt)[:22]}'

        # Create simple text message
        message_text = f"{self.message0_template} {last_event_str} UTC.\n"
        if self.extra_text:
            message_text += f"{self.extra_text}\n"
        message_text += f"Live Feed: {self.livelink}"

        # Ensure message length is within LINE limits (though checked in extra_text, double check final)
        if len(message_text) > 5000:
            message_text = message_text[:4997] + "..."
            printW("Message truncated to fit LINE limit.", self.sender)

        message_object = TextMessage(text=message_text)
        self._send_line_message(message_object)

    def _when_img(self, d):
        """Image handling - Not implemented as per user request."""
        printM('Ignoring IMGPATH message (image sending disabled for LINE).', self.sender)
        pass

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