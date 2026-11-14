## Server log

When a page is requested, the server writes one line to its log: the time, the address requested, and the status and size of the response. Your IP address is shortened before it is written, for IPv4 to the /{ipv4} network and for IPv6 to the /{ipv6} network: the rest of the address is set to zero and not stored. The log is kept for {days} days. It is deleted day by day, because the system journal starts a new file every day and removes only whole files. The purpose is the operation and security of the website.

If someone fails to sign in to the admin area at /admin, or a sign-in through single sign-on is refused, the server writes the full IP address to the system log. It is kept for the same time and concerns only people who work on this website.
