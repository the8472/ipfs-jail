include /etc/ipfs/jail/custom-blacklist.profile

# noblacklist /sbin/dhclient
blacklist /etc
blacklist /root
blacklist /media
# blacklist /sbin
blacklist /usr/sbin
tmpfs /tmp
caps.drop all
# protocol unix,inet,inet6
seccomp
private-dev
shell none
