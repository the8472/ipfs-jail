# ipfs-jail

systemd unit file + firejail wrapper for ipfs-daemon to launch jailed daemon as service.

Ipfs is still experimental and needs filessystem and internet access and also configurable via remote access. To sleep more soundly I prefer it to run in a sandbox.

Advantages over a docker container are out-of-the-box seccomp filtering of unneeded system calls the ability to use whichever binaries are installed on the host system instead of having to manage container images.

 

## Prerequisites

* a bridge network device (default: br0), bridged to a physical device
* a separate user (default: ipfs) with a `.ipfs/` repository in his home. a symlink will suffice
* firejail
* dhclient
* iptables
* ipfs executable in `PATH`

## Default configuration

* allows connections from/to the public internet
* blocks connections to the local lan; allows from the local lan
* rules for v4 and v6
* blacklists common directories containing private data; if you have zpools/btrfs mounted outside /media add them to the custom blacklist

optional: netfilter rules for throttling or blocking outgoing ipv4 connections to put less stress on NATs/conntrack 