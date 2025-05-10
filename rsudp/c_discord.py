import sys
import os
import time
import requests # Dependency: Make sure 'requests' is installed
import json # For creating embed objects
from datetime import timezone, timedelta # For JST conversion

from rsudp import printM, printW, printE, helpers
import rsudp.raspberryshake as rs
from rsudp.test import TEST

class Discorder(rs.ConsumerThread):
    """
    Consumer thread to send seismic event notifications to Discord via Incoming Webhook.

    Supports sending messages as simple text or rich embeds, and optionally attaching images.

    Inherits from rsudp.raspberryshake.ConsumerThread.

    Args:
        webhook_url (str): The Discord Incoming Webhook URL.
        q (queue.Queue): The queue object for receiving messages.
        use_embed (bool, optional): If True, sends messages as embeds. Otherwise, sends as simple text. Defaults to True.
        send_images (bool, optional): If True, attaches images from IMGPATH messages. Defaults to True.
        extra_text (str, optional): Additional text to append to the notification message (used in embed description or text content). Defaults to False.
        testing (bool, optional): If True, runs in testing mode without actually sending messages. Defaults to False.
    """
    def __init__(self, webhook_url, q=False, use_embed=True, send_images=True, extra_text=False, testing=False):
        super().__init__()
        if not q:
            printE('Discorder: Queue is required.', self.sender)
            raise ValueError("Queue object is required for Discorder")
        if not webhook_url:
            printE('Discorder: Webhook URL is required.', self.sender)
            raise ValueError("Webhook URL is required for Discorder")

        self.queue = q
        self.sender = 'Discord'
        self.alive = True
        self.webhook_url = webhook_url
        self.use_embed = use_embed
        self.send_images = send_images
        self.testing = testing
        self.fmt = '%Y-%m-%d %H:%M:%S.%f' # Original format for parsing
        self.jst_fmt = '%Y年%m月%d日 %H時%M分%S秒 (JST)' # JST format for display
        self.region_text = f"{rs.region.title()}地方" if rs.region else "不明な地域"
        # Resolve extra text, Discord limits are generally large, especially for embeds
        self.extra_text = helpers.resolve_extra_text(extra_text, max_len=2000, sender=self.sender) # Embed description limit is 4096, content is 2000

        self.livelink = f'https://stationview.raspberryshake.org/#?net={rs.net}&sta={rs.stn}'
        # self.message0 is now generated dynamically in _when_alarm
        self.last_message_payload = None
        self.last_message_files = None

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

    def _send_message(self, payload=None, files=None):
        """Internal helper to send messages/files to the Discord webhook."""
        response = None
        headers = {} # No special headers needed unless using specific features

        # Discord expects JSON payload in 'payload_json' field when sending files
        data_payload = {}
        if files and payload:
            data_payload['payload_json'] = json.dumps(payload)
        elif payload:
             # If no files, send payload directly as JSON
             headers['Content-Type'] = 'application/json'


        try:
            printM(f'Sending message to Discord...', sender=self.sender)
            if payload:
                 printM(f'Payload: {json.dumps(payload, indent=2)}', sender=self.sender) # Log payload for debugging
            if files:
                 printM(f'Files: {[f for f in files]}', sender=self.sender) # Log file names

            if not self.testing:
                if files:
                    # Send as multipart/form-data
                    response = requests.post(self.webhook_url, data=data_payload, files=files, timeout=20) # Longer timeout for uploads
                elif payload:
                     # Send as application/json
                     response = requests.post(self.webhook_url, headers=headers, json=payload, timeout=10)
                else:
                     printW("Nothing to send (no payload or files).", sender=self.sender)
                     return # Nothing to do

                response.raise_for_status() # Raise HTTPError for bad responses (4xx or 5xx)
                printM('Message/File sent successfully.', sender=self.sender)
                # Update test status on success
                if payload and 'embeds' in payload: TEST['c_discord'][1] = True
                if files: TEST['c_discordimg'][1] = True

            else:
                printM('Testing mode: Message/File prepared but not sent.', sender=self.sender)
                if payload and 'embeds' in payload: TEST['c_discord'][1] = True
                if files: TEST['c_discordimg'][1] = True
                # Add 'c_discord' and 'c_discordimg' to rsudp.test.TEST dictionary if not present
                # Example: TEST = {..., 'c_discord': ['Discord embed message', False], 'c_discordimg': ['Discord image attachment', False]}

        except requests.exceptions.RequestException as e:
            printE(f'Could not send message/file - {e}', sender=self.sender)
            # Implement retry logic if desired (consider retry for network errors, not necessarily 4xx errors)
            # Basic retry example:
            printE('Attempting retry in 5 seconds...', sender=self.sender, spaces=True)
            time.sleep(5)
            try:
                if not self.testing:
                    if files:
                        response = requests.post(self.webhook_url, data=data_payload, files=files, timeout=30) # Longer retry timeout
                    elif payload:
                        response = requests.post(self.webhook_url, headers=headers, json=payload, timeout=15)
                    else:
                         return # Still nothing to send

                    response.raise_for_status()
                    printM('Message/File sent successfully on retry.', sender=self.sender)
                    if payload and 'embeds' in payload: TEST['c_discord'][1] = True
                    if files: TEST['c_discordimg'][1] = True
                else:
                     printM('Testing mode: Retry prepared but not sent.', sender=self.sender)
                     if payload and 'embeds' in payload: TEST['c_discord'][1] = True
                     if files: TEST['c_discordimg'][1] = True
            except requests.exceptions.RequestException as e2:
                printE(f'Final failure to send message/file - {e2}', sender=self.sender)
                if self.testing:
                    if payload and 'embeds' in payload: TEST['c_discord'][1] = False
                    if files: TEST['c_discordimg'][1] = False

        self.last_message_payload = payload
        self.last_message_files = files # Store file info if needed later

    def _when_alarm(self, d):
        """Actions to take when an ALARM message is received."""
        event_time_utc_obspy = helpers.fsec(helpers.get_msg_time(d))
        
        # Convert to JST
        py_datetime_utc = event_time_utc_obspy.datetime
        jst_tz = timezone(timedelta(hours=9))
        event_time_jst = py_datetime_utc.astimezone(jst_tz)
        jst_time_str = event_time_jst.strftime(self.jst_fmt)

        payload = {}
        description_parts = [f"発生時刻: **{jst_time_str}**"]
        if self.extra_text:
            description_parts.append(str(self.extra_text).strip())

        if self.use_embed:
            embed = {
                "title": "【地震イベント検知】",
                "description": "\n".join(description_parts),
                "color": 15277667, # Discord Orange
                "fields": [
                    {"name": "詳細リンク", "value": f"[StationView]({self.livelink})", "inline": False},
                    {"name": "ステーションID", "value": f"{rs.net}.{rs.stn}", "inline": True},
                    {"name": "地域", "value": self.region_text, "inline": True}
                ],
                "timestamp": event_time_jst.isoformat() # Use JST for timestamp
            }
            payload['embeds'] = [embed]
        else:
            # Simple text message
            message_parts = [
                "【地震イベント検知】",
                f"発生時刻: {jst_time_str}\n",
                "詳細は以下のリンクをご確認ください:",
                f"<{self.livelink}>" # Angle brackets for auto-linking
            ]
            if self.extra_text:
                message_parts.append(f"\n{str(self.extra_text).strip()}")
            
            message_parts.extend([
                "\n---",
                "観測点情報:",
                f"ステーションID: {rs.net}.{rs.stn}",
                f"地域: {self.region_text}"
            ])
            payload['content'] = "\n".join(message_parts)

        self._send_message(payload=payload)

    def _when_img(self, d):
        """Actions to take when an IMGPATH message is received."""
        if not self.send_images:
            printM('Ignoring IMGPATH message (image sending disabled).', sender=self.sender)
            return

        imgpath = helpers.get_msg_path(d)
        if not os.path.exists(imgpath):
            printW(f'Could not find image: {imgpath}', sender=self.sender)
            return

        try:
            with open(imgpath, 'rb') as image_file:
                files = {'file': (os.path.basename(imgpath), image_file.read(), 'image/png')} # Assume PNG, adjust if needed
                # Optionally add a simple message or embed with the image
                payload = {'content': f"地震イベントの画像 ({rs.net}.{rs.stn})"}
                # Or create an embed describing the image
                # embed = {"title": "地震イベント画像", "image": {"url": f"attachment://{os.path.basename(imgpath)}"}}
                # payload = {'embeds': [embed]}

                self._send_message(payload=payload, files=files)

        except IOError as e:
            printE(f"Could not open or read image file {imgpath}: {e}", sender=self.sender)
        except Exception as e:
            printE(f"An unexpected error occurred while processing image {imgpath}: {e}", sender=self.sender)


    def run(self):
        """Main loop to process messages from the queue."""
        while self.alive:
            d = self.getq()

            if 'ALARM' in str(d):
                self._when_alarm(d)
            elif 'IMGPATH' in str(d):
                self._when_img(d)

# Example of how to add to rsudp.test.TEST (in rsudp/test.py)
# TEST = {
#     ... existing tests ...
#     'c_discord': ['Discord embed message', False],
#     'c_discordimg': ['Discord image attachment', False],
# }