import sys
import time
import requests  # Dependency: Make sure 'requests' is installed

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
    def __init__(self, webhook_url, q=False, extra_text=False, testing=False):
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
        self.fmt = '%Y-%m-%d %H:%M:%S.%f'
        self.region = f' - region: {rs.region.title()}' if rs.region else ''
        # Resolve extra text, ensuring it fits within potential Google Chat limits (though usually generous for text)
        self.extra_text = helpers.resolve_extra_text(extra_text, max_len=4000, sender=self.sender) # Google Chat limit is large

        self.livelink = f'live feed ➡️ https://stationview.raspberryshake.org/#?net={rs.net}&sta={rs.stn}'
        self.message0 = f'(Raspberry Shake station {rs.net}.{rs.stn}{self.region}) Event detected at'
        self.last_message = False

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
        event_time = helpers.fsec(helpers.get_msg_time(d))
        last_event_str = f'{event_time.strftime(self.fmt)[:22]}' # Format time
        message = f'{self.message0} {last_event_str} UTC{self.extra_text} - {self.livelink}'

        self._send_message(message)

    def run(self):
        """Main loop to process messages from the queue."""
        while self.alive:
            d = self.getq()

            if 'ALARM' in str(d):
                self._when_alarm(d)

            # Ignore IMGPATH messages as per the final design
            elif 'IMGPATH' in str(d):
                printM('Ignoring IMGPATH message (image sending disabled).', sender=self.sender)
                pass # Explicitly do nothing

# Example of how to add to rsudp.test.TEST (in rsudp/test.py)
# TEST = {
#     ... existing tests ...
#     'c_googlechat': ['Google Chat message sending', False],
# }