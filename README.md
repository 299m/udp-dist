# udp-dist
A simple UDP-based file distribution tool.
This is a WIP.

## Using udp-dist through a TCP tunnel

When tunnelled through TCP, the UDP message boundaries could be lost.
The distributor should be able to work them out, provided no messages are dropped. For this reason the distributor
should be run on the same machine as the TCP tunnel ends.

The plan is to add some sort of re-sync message ... but that's for later.