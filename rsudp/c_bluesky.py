import os, sys
import time
import rsudp.raspberryshake as rs
from rsudp import printM, printW, printE, helpers
from rsudp.test import TEST
from atproto import Client


class Blueskyer(rs.ConsumerThread):
	'''
	.. versionadded:: 1.0.2

		The option to add extra text to Bluesky posts using the :code:`"extra_text"`
		setting in settings files built by this version and later.

	.. |bluesky| raw:: html

		<a href="https://bsky.app" target="_blank">Bluesky</a>

	Bluesky is a decentralized social media platform sometimes used for quickly
	distributing public alert information.

	:param str username: Bluesky username (email)
	:param str password: Bluesky app password
	:param bool post_images: whether or not to send images. if False, only alerts will be sent.
	:type extra_text: bool or str
	:param extra_text: 300 additional characters to post as part of the Bluesky message (longer messages will be truncated).
	:param queue.Queue q: queue of data and messages sent by :class:`rsudp.c_consumer.Consumer`
	'''
	def __init__(self, username, password,
				 q=False, post_images=False, extra_text=False, testing=False
				 ):
		"""
		Initialize the process
		"""
		super().__init__()
		self.queue = q
		self.sender = 'Blueskyer'
		self.alive = True
		self.post_images = post_images
		self.testing = testing
		self.fmt = '%Y-%m-%d %H:%M:%S.%f'
		self.region = ' - region: %s' % rs.region.title() if rs.region else ''
		self.username = username
		self.password = password
		self.last_message = False

		self.extra_text = helpers.resolve_extra_text(extra_text, max_len=300, sender=self.sender)

		self.auth()

		# Japanese messages only
		self.livelink = u'ライブフィード ➡️ https://stationview.raspberryshake.org/#?net=%s&sta=%s' % (rs.net, rs.stn)
		self.message0 = u'(#RaspberryShake ステーション %s.%s%s) 強い揺れを検知しました' % (rs.net, rs.stn, self.region)
		self.message1 = u'(#RaspberryShake ステーション %s.%s%s) 強い揺れの画像' % (rs.net, rs.stn, self.region)

		printM('Starting.', self.sender)


	def auth(self):
		if not self.testing:
			try:
				self.client = Client()
				self.client.login(self.username, self.password)
				printM('Successfully authenticated with Bluesky', self.sender)
			except Exception as e:
				printE(f'Failed to authenticate with Bluesky: {e}', self.sender)
				self.client = None
		else:
			printW('The Bluesky module will not post to Bluesky in Testing mode.',
					self.sender, announce=False)
			self.client = None


	def getq(self):
		d = self.queue.get()
		self.queue.task_done()

		if 'TERM' in str(d):
			self.alive = False
			printM('Exiting.', self.sender)
			sys.exit()
		else:
			return d


	def _when_alarm(self, d):
		'''
		Send a Bluesky post when you get an ``ALARM`` message.

		:param bytes d: queue message
		'''
		event_time = helpers.fsec(helpers.get_msg_time(d))
		self.last_event_str = '%s' % (event_time.strftime(self.fmt)[:22])
		message = '%s %s UTC%s - %s' % (self.message0, self.last_event_str, self.extra_text, self.livelink)
		response = None
		try:
			printM('Bluesky post: %s' % (message), sender=self.sender)
			if not self.testing and self.client:
				response = self.client.send_post(text=message)
				post_url = f"https://bsky.app/profile/{self.client.me.did}/post/{response.uri.rkey}"
				printM('Bluesky post URL: %s' % post_url)
			if self.testing:
				TEST['c_bluesky'][1] = True

		except Exception as e:
			printE('could not send alert to Bluesky - %s' % (e))
			try:
				printE('Waiting 5 seconds and trying to send post again...', sender=self.sender, spaces=True)
				time.sleep(5.1)
				printM('Bluesky post: %s' % (message), sender=self.sender)
				if not self.testing and self.client:
					self.auth()
					response = self.client.send_post(text=message)
					post_url = f"https://bsky.app/profile/{self.client.me.did}/post/{response.uri.rkey}"
					printM('Bluesky post URL: %s' % post_url)
			except Exception as e:
				printE('could not send alert to Bluesky - %s' % (e))
				response = None

		self.last_message = message



	def _when_img(self, d):
		'''
		Send a Bluesky post with an image when you get an ``IMGPATH`` message.

		:param bytes d: queue message
		'''
		if self.post_images:
			imgpath = helpers.get_msg_path(d)
			imgtime = helpers.fsec(helpers.get_msg_time(d))
			message = '%s %s UTC%s' % (self.message1, imgtime.strftime(self.fmt)[:22], self.extra_text)
			response = None
			printM('Image post: %s' % (message), sender=self.sender)
			if not self.testing and self.client:
				if os.path.exists(imgpath):
					try:
						printM('Uploading image to Bluesky %s' % (imgpath), self.sender)
						with open(imgpath, 'rb') as f:
							img_data = f.read()

						# Upload the image to Bluesky
						upload_response = self.client.upload_blob(img_data)

						# Create a post with the image
						response = self.client.send_post(
							text=message,
							images=[upload_response]
						)
						post_url = f"https://bsky.app/profile/{self.client.me.did}/post/{response.uri.rkey}"
						printM('Bluesky post URL: %s' % post_url)
					except Exception as e:
						printE('could not send multimedia post to Bluesky - %s' % (e))
						try:
							printM('Waiting 5 seconds and trying to send post again...', sender=self.sender)
							time.sleep(5.1)
							self.auth()
							printM('Uploading image to Bluesky (2nd try) %s' % (imgpath), self.sender)
							with open(imgpath, 'rb') as f:
								img_data = f.read()

							# Upload the image to Bluesky
							upload_response = self.client.upload_blob(img_data)

							# Create a post with the image
							response = self.client.send_post(
								text=message,
								images=[upload_response]
							)
							post_url = f"https://bsky.app/profile/{self.client.me.did}/post/{response.uri.rkey}"
							printM('Bluesky post URL: %s' % post_url)

						except Exception as e:
							printE('could not send multimedia post to Bluesky (2nd try) - %s' % (e))
							response = None

				else:
					printM('Could not find image: %s' % (imgpath), sender=self.sender)
			else:
				TEST['c_bskyimg'][1] = True

		self.last_message = message

	def run(self):
		"""
		Reads data from the queue and posts to Bluesky if it sees an ALARM or IMGPATH message
		"""
		while True:
			d = self.getq()

			if 'ALARM' in str(d):
				self._when_alarm(d)

			elif 'IMGPATH' in str(d):
				self._when_img(d)
